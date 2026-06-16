package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"github.com/Ketan6969/capy/internal/dom"
	"github.com/Ketan6969/capy/internal/engine"
	bnetwork "github.com/Ketan6969/capy/internal/network"
)

type CompareStats struct {
	Requests int
	DOMNodes int
	JSErrors int
	Title    string
}

func runCompare(targetURL string, timeout time.Duration) {
	fmt.Println("========================================")
	fmt.Println("Capy Runtime Comparison Report")
	fmt.Println("========================================")
	fmt.Printf("\nURL:\n%s\n\n", targetURL)

	bStats := runCapyForCompare(targetURL, timeout)
	cStats := runChromiumForCompare(targetURL, timeout)

	networkMatch := float64(bStats.Requests) / math.Max(1.0, float64(cStats.Requests))
	if networkMatch > 1.0 {
		networkMatch = 1.0
	}

	domMatch := float64(bStats.DOMNodes) / math.Max(1.0, float64(cStats.DOMNodes))
	if domMatch > 1.0 {
		domMatch = 1.0
	}

	contentMatch := 0.0
	if cStats.Title != "" && bStats.Title != "" {
		if cStats.Title == bStats.Title {
			contentMatch = 1.0
		} else {
			contentMatch = 0.5
		}
	}
	jsMatch := 1.0
	if cStats.JSErrors == 0 && bStats.JSErrors > 0 {
		jsMatch = math.Max(0.0, 1.0-float64(bStats.JSErrors)*0.1)
	}

	if cStats.Requests == 0 {
		networkMatch = 1.0
	}
	if cStats.DOMNodes == 0 {
		domMatch = 1.0
	}
	if cStats.Title == "" && bStats.Title == "" {
		contentMatch = 1.0
	}

	overall := (networkMatch * 0.20) + (domMatch * 0.30) + (contentMatch * 0.30) + (jsMatch * 0.20)

	verdict := "Incompatible"
	score := overall * 100
	if score >= 95 {
		verdict = "Excellent"
	} else if score >= 85 {
		verdict = "Compatible"
	} else if score >= 70 {
		verdict = "Partial Compatibility"
	} else if score >= 50 {
		verdict = "Major Issues"
	}

	fmt.Printf("Network Match:     %.0f%%\n", networkMatch*100)
	fmt.Printf("DOM Match:         %.0f%%\n", domMatch*100)
	fmt.Printf("Content Match:     %.0f%%\n", contentMatch*100)
	fmt.Printf("JavaScript Match:  %.0f%%\n\n", jsMatch*100)

	fmt.Printf("Overall Match:     %.0f%%\n\n", overall*100)

	fmt.Printf("Verdict:\n%s\n\n", verdict)

	fmt.Println("Differences:")
	fmt.Println()
	diffFound := false
	if bStats.Requests != cStats.Requests {
		fmt.Printf("- Network requests difference: %d\n", bStats.Requests-cStats.Requests)
		diffFound = true
	}
	if bStats.DOMNodes != cStats.DOMNodes {
		fmt.Printf("- DOM node difference: %d\n", bStats.DOMNodes-cStats.DOMNodes)
		diffFound = true
	}
	if bStats.JSErrors != cStats.JSErrors {
		fmt.Printf("- JS errors difference: %d\n", bStats.JSErrors-cStats.JSErrors)
		diffFound = true
	}
	if !diffFound {
		fmt.Println("- No major differences detected.")
	}

	fmt.Println()
	fmt.Println("Statistics:")
	fmt.Println()

	fmt.Println("Chromium:")
	fmt.Printf("  Requests: %d\n", cStats.Requests)
	fmt.Printf("  DOM Nodes: %d\n", cStats.DOMNodes)
	fmt.Printf("  JS Errors: %d\n\n", cStats.JSErrors)

	fmt.Println("Capy:")
	fmt.Printf("  Requests: %d\n", bStats.Requests)
	fmt.Printf("  DOM Nodes: %d\n", bStats.DOMNodes)
	fmt.Printf("  JS Errors: %d\n", bStats.JSErrors)
}

func runChromiumForCompare(urlStr string, timeout time.Duration) CompareStats {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	var stats CompareStats

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev.(type) {
		case *network.EventRequestWillBeSent:
			stats.Requests++
		case *cdpruntime.EventExceptionThrown:
			stats.JSErrors++
		}
	})

	var nodeCount int
	var title string
	err := chromedp.Run(ctx,
		network.Enable(),
		cdpruntime.Enable(),
		network.SetExtraHTTPHeaders(network.Headers{"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"}),
		chromedp.ActionFunc(func(c context.Context) error {
			_, _, _, _, err := page.Navigate(urlStr).Do(c)
			return err
		}),
		chromedp.Sleep(8*time.Second), // 8 seconds for robust loading
		chromedp.Evaluate(`document.title`, &title),
		chromedp.Evaluate(`document.querySelectorAll('*').length`, &nodeCount),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Chromium Run Error: %v\n", err)
	}

	stats.DOMNodes = nodeCount
	stats.Title = title
	return stats
}

func runCapyForCompare(urlStr string, timeout time.Duration) CompareStats {
	var stats CompareStats

	htmlContent, err := loadHTML(urlStr)
	if err != nil {
		return stats
	}

	doc, err := dom.ParseHTML(htmlContent)
	if err != nil {
		return stats
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	runCtx := engine.NewContext(ctx)
	defer runCtx.Close()

	nm := bnetwork.NewNetworkManager()
	dom.SetupDOM(runCtx.VM(), doc, urlStr)
	dom.SetupCookies(runCtx.VM(), nm.CookieJar(), urlStr)
	nm.SetupNetworkInstrumented(runCtx, &stats.Requests)

	var jsErrors []string
	runCtx.SetErrorCollector(func(msg string) {
		jsErrors = append(jsErrors, msg)
	})

	if err := bootstrapPageScripts(runCtx, doc, urlStr); err != nil {
		jsErrors = append(jsErrors, err.Error())
	}

	// Wait for stabilization
	go func() {
		time.Sleep(2 * time.Second)
		cancel()
	}()
	runCtx.EventLoop()

	stats.JSErrors = len(jsErrors)

	val := runCtx.VM().Get("document")
	if val != nil {
		if docNode, ok := val.Export().(*dom.Node); ok {
			stats.DOMNodes = countDOMNodes(docNode)
			titleNode := docNode.QuerySelector("title")
			if titleNode != nil {
				stats.Title = titleNode.GetTextContent()
			}
		}
	}

	stats.Requests++ // Account for initial loadHTML
	return stats
}
