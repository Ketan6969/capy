package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/browserless/runtime/internal/dom"
	"github.com/browserless/runtime/internal/engine"
	"github.com/browserless/runtime/internal/network"
	"github.com/dop251/goja"
)

// Helpers duplicated from cmd/browserless for testing
func loadHTML(pathOrURL string) (string, error) {
	if strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://") {
		resp, err := http.Get(pathOrURL)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		return string(b), err
	}
	b, err := os.ReadFile(pathOrURL)
	return string(b), err
}

func bootstrapPageScripts(runCtx *engine.Context, doc *dom.Node, baseRef string) error {
	if doc == nil {
		return nil
	}
	scripts := doc.GetElementsByTagName("script")
	for i, scriptNode := range scripts {
		scriptType := scriptNode.GetAttribute("type")
		if !isExecutableScript(scriptType) {
			continue
		}

		src := strings.TrimSpace(scriptNode.GetAttribute("src"))
		scriptName := fmt.Sprintf("inline-script-%d.js", i+1)
		content := strings.TrimSpace(stripCDATA(scriptNode.GetTextContent()))

		if src != "" {
			resolved, err := resolveScriptRef(baseRef, src)
			if err != nil {
				return err
			}
			body, err := loadHTML(resolved)
			if err != nil {
				return err
			}
			scriptName = resolved
			content = stripCDATA(body)
		}

		if strings.TrimSpace(content) == "" {
			continue
		}
		runCtx.RunScript(scriptName, content)
	}
	return nil
}

func isExecutableScript(scriptType string) bool {
	switch strings.ToLower(strings.TrimSpace(scriptType)) {
	case "", "text/javascript", "application/javascript", "text/ecmascript", "module":
		return true
	default:
		return false
	}
}

func stripCDATA(text string) string {
	replacer := strings.NewReplacer("/* <![CDATA[ */", "", "/* ]]> */", "", "//<![CDATA[", "", "//]]>", "")
	return replacer.Replace(text)
}

func resolveScriptRef(baseRef, scriptRef string) (string, error) {
	if strings.HasPrefix(scriptRef, "http://") || strings.HasPrefix(scriptRef, "https://") {
		return scriptRef, nil
	}
	if strings.HasPrefix(baseRef, "http://") || strings.HasPrefix(baseRef, "https://") {
		baseURL, _ := url.Parse(baseRef)
		refURL, _ := url.Parse(scriptRef)
		return baseURL.ResolveReference(refURL).String(), nil
	}
	basePath := baseRef
	if basePath == "" {
		basePath = "."
	}
	if strings.HasPrefix(scriptRef, "/") {
		return scriptRef, nil
	}
	if info, err := os.Stat(basePath); err == nil && !info.IsDir() {
		basePath = filepath.Dir(basePath)
	} else if filepath.Ext(basePath) != "" {
		basePath = filepath.Dir(basePath)
	}
	return filepath.Clean(filepath.Join(basePath, scriptRef)), nil
}

func runTestEnv(t *testing.T, htmlPath, extScript string, wait time.Duration, evalBeforeLoop bool) []string {
	htmlContent, err := loadHTML(htmlPath)
	if err != nil {
		t.Fatalf("Failed to load HTML: %v", err)
	}

	doc, err := dom.ParseHTML(htmlContent)
	if err != nil {
		t.Fatalf("Failed to parse HTML: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	t.Cleanup(cancel)

	runCtx := engine.NewContext(ctx)
	defer runCtx.Close()

	nm := network.NewNetworkManager()
	dom.SetupDOM(runCtx.VM(), doc, htmlPath)
	dom.SetupCookies(runCtx.VM(), nm.CookieJar(), htmlPath)

	var reqCount int
	nm.SetupNetworkInstrumented(runCtx, &reqCount)

	var logs []string

	runCtx.SetErrorCollector(func(msg string) {
		fmt.Printf("JS Error Collected: %s\n", msg)
	})

	runCtx.Enqueue(func() {
		consoleVal := runCtx.VM().Get("console")
		if consoleVal != nil {
			consoleObj := consoleVal.ToObject(runCtx.VM())
			consoleObj.Set("log", func(call goja.FunctionCall) goja.Value {
				var parts []string
				for _, arg := range call.Arguments {
					parts = append(parts, fmt.Sprintf("%v", arg.Export()))
				}
				logs = append(logs, strings.Join(parts, " "))
				return goja.Undefined()
			})
		}
	})

	if err := bootstrapPageScripts(runCtx, doc, htmlPath); err != nil {
		t.Fatalf("Bootstrap error: %v", err)
	}

	if extScript != "" && evalBeforeLoop {
		scriptBody, err := os.ReadFile(extScript)
		if err != nil {
			t.Fatalf("Failed to load script: %v", err)
		}

		runCtx.WgAdd(1)
		runCtx.Enqueue(func() {
			defer runCtx.WgDone()
			scriptStr := "(function() {\n" + string(scriptBody) + "\n})();"
			_, err := runCtx.VM().RunString(scriptStr)
			if err != nil {
				fmt.Printf("Script error in %s: %v\n", extScript, err)
			}
		})
	}

	if extScript != "" && !evalBeforeLoop {
		scriptBody, err := os.ReadFile(extScript)
		if err != nil {
			t.Fatalf("Failed to load script: %v", err)
		}

		runCtx.WgAdd(1)
		go func() {
			defer runCtx.WgDone()
			time.Sleep(500 * time.Millisecond)
			runCtx.WgAdd(1)
			runCtx.Enqueue(func() {
				defer runCtx.WgDone()
				scriptStr := "(function() {\n" + string(scriptBody) + "\n})();"
				fmt.Printf("Running delayed script for %s\n", extScript)
				_, err := runCtx.VM().RunString(scriptStr)
				fmt.Printf("Delayed script finished: err=%v\n", err)
			})
		}()
	}

	runCtx.EventLoop()

	return logs
}

func TestRealReactHydration(t *testing.T) {
	// React needs a little time to hydrate and for the setTimeout to fire.
	// We wait for the event loop to naturally exit or timeout, then evaluate script.
	logs := runTestEnv(t, "../engine-smoke/real-react/index.html", "../engine-smoke/real-react/script.js", 5*time.Second, false)

	found := false
	for _, l := range logs {
		if strings.Contains(l, "REACT RESULT: You liked this.") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'REACT RESULT: You liked this.', but logs were: %v", logs)
	}
}

func TestEventAndCookies(t *testing.T) {
	logs := runTestEnv(t, "../engine-smoke/events-cookies/index.html", "", 5*time.Second, false)

	foundCookie := false
	foundTrace := false
	for _, l := range logs {
		if strings.Contains(l, "COOKIE: session_id=12345") {
			foundCookie = true
		}
		if strings.Contains(l, "EVENT TRACE: parent-capture -> child-target -> parent-bubble") {
			foundTrace = true
		}
	}

	if !foundCookie {
		t.Errorf("Did not find correct cookie log. Logs: %v", logs)
	}
	if !foundTrace {
		t.Errorf("Did not find correct event trace log. Logs: %v", logs)
	}
}

func TestShopifyMock(t *testing.T) {
	logs := runTestEnv(t, "../engine-smoke/shopify/index.html", "../engine-smoke/shopify/script.js", 5*time.Second, true)

	expectedResults := []string{
		"[RESULT] Page Loaded: PASS",
		"[RESULT] JS Executed: PASS",
		"[RESULT] DOM Updated: PASS",
		"[RESULT] Fetch Worked: PASS",
		"[RESULT] Timers Worked: PASS",
		"[RESULT] Events Worked: PASS",
		"[RESULT] Storage Worked: PASS",
	}

	for _, expected := range expectedResults {
		found := false
		for _, l := range logs {
			if strings.Contains(l, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Did not find log %q. Logs: %v", expected, logs)
		}
	}
}

func TestVueHydration(t *testing.T) {
	logs := runTestEnv(t, "../engine-smoke/vue/index.html", "../engine-smoke/vue/script.js", 5*time.Second, true)

	expectedResults := []string{
		"[RESULT] Page Loaded: PASS",
		"[RESULT] JS Executed: PASS",
		"[RESULT] DOM Updated: PASS",
	}

	for _, expected := range expectedResults {
		found := false
		for _, l := range logs {
			if strings.Contains(l, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Did not find log %q. Logs: %v", expected, logs)
		}
	}
}
