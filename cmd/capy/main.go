package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/Ketan6969/capy/internal/dom"
	"github.com/Ketan6969/capy/internal/engine"
	"github.com/Ketan6969/capy/internal/network"
	"github.com/Ketan6969/capy/internal/semantic"
)

func setupLogging(format string, debug bool) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if strings.ToLower(format) == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	slog.SetDefault(slog.New(handler))
}

func main() {
	startTime := time.Now()

	// ── Subcommand routing ──────────────────────────────────────────────────
	// Find subcommand to support global flags placed before or after the subcommand
	subcmd := ""
	subcmdIdx := -1
	for i, arg := range os.Args[1:] {
		if arg == "audit" || arg == "compare" || arg == "serve" {
			subcmd = arg
			subcmdIdx = i + 1
			break
		}
	}

	if subcmd == "audit" {
		auditCmd := flag.NewFlagSet("audit", flag.ExitOnError)
		auditTimeout := auditCmd.Int("timeout", 15, "Timeout for script execution in seconds")
		auditDebug := auditCmd.Bool("debug", false, "Enable verbose debug logging")
		auditFormat := auditCmd.String("log-format", "text", "Log format (text or json)")
		
		// Parse all args except the subcommand itself
		argsForCmd := append(os.Args[1:subcmdIdx], os.Args[subcmdIdx+1:]...)
		auditCmd.Parse(argsForCmd)

		setupLogging(*auditFormat, *auditDebug)

		if auditCmd.NArg() < 1 {
			fmt.Fprintln(os.Stderr, "Usage: capy audit [options] <url>")
			auditCmd.PrintDefaults()
			os.Exit(1)
		}
		runAudit(auditCmd.Arg(0), time.Duration(*auditTimeout)*time.Second)
		return
	}

	if subcmd == "compare" {
		compareCmd := flag.NewFlagSet("compare", flag.ExitOnError)
		compareTimeout := compareCmd.Int("timeout", 30, "Timeout for comparison in seconds")
		compareDebug := compareCmd.Bool("debug", false, "Enable verbose debug logging")
		compareFormat := compareCmd.String("log-format", "text", "Log format (text or json)")
		
		argsForCmd := append(os.Args[1:subcmdIdx], os.Args[subcmdIdx+1:]...)
		compareCmd.Parse(argsForCmd)

		setupLogging(*compareFormat, *compareDebug)

		if compareCmd.NArg() < 1 {
			fmt.Fprintln(os.Stderr, "Usage: capy compare [options] <url>")
			compareCmd.PrintDefaults()
			os.Exit(1)
		}
		runCompare(compareCmd.Arg(0), time.Duration(*compareTimeout)*time.Second)
		return
	}

	if subcmd == "serve" {
		serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
		portFlag := serveCmd.Int("port", 8080, "Port to run the HTTP API server on")
		
		argsForCmd := append(os.Args[1:subcmdIdx], os.Args[subcmdIdx+1:]...)
		serveCmd.Parse(argsForCmd)
		
		startServer(*portFlag)
		return
	}

	// Normal run
	scriptFile := flag.String("file", "", "Path to the JavaScript file to execute")
	scriptEval := flag.String("eval", "", "JavaScript code to execute directly")
	htmlPath := flag.String("html", "", "Path or URL to the initial HTML content (optional)")
	printDOM := flag.Bool("print-dom", false, "Print the final DOM HTML after execution")
	semanticFlag := flag.Bool("semantic", false, "Print the semantic page graph as JSON after execution")
	timeoutSec := flag.Int("timeout", 10, "Timeout for script execution in seconds")
	bootstrapScripts := flag.Bool("bootstrap-page-scripts", true, "Load and execute script tags from the input HTML before running the user script")
	recordPath := flag.String("record", "", "Path to save recorded network requests")
	replayPath := flag.String("replay", "", "Path to rules file to replay network requests directly")
	statsFlag := flag.Bool("stats", false, "Print execution time and memory usage statistics")
	debugFlag := flag.Bool("debug", false, "Enable verbose debug logging")
	formatFlag := flag.String("log-format", "text", "Log format (text or json)")

	flag.Parse()
	setupLogging(*formatFlag, *debugFlag)

	// If replay mode is requested, run it directly and exit
	if *replayPath != "" {
		nm := network.NewNetworkManager()
		replayData, err := nm.Optimizer.Replay(*replayPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Replay error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(replayData))
		if *statsFlag {
			printStats(startTime, nil)
		}
		os.Exit(0)
	}

	if *scriptFile == "" && *scriptEval == "" {
		fmt.Fprintln(os.Stderr, "Error: either -file or -eval must be specified")
		flag.Usage()
		os.Exit(1)
	}

	var htmlContent string
	if *htmlPath != "" {
		var err error
		htmlContent, err = loadHTML(*htmlPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading HTML: %v\n", err)
			os.Exit(1)
		}
	}

	var scriptContent string
	scriptName := "eval"
	if *scriptFile != "" {
		content, err := os.ReadFile(*scriptFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading script file: %v\n", err)
			os.Exit(1)
		}
		scriptContent = string(content)
		scriptName = *scriptFile
	} else {
		scriptContent = *scriptEval
	}

	// Parse initial DOM
	var doc *dom.Node
	if htmlContent != "" {
		var err error
		doc, err = dom.ParseHTML(htmlContent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing HTML: %v\n", err)
			os.Exit(1)
		}
	}

	// Set up execution context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutSec)*time.Second)
	defer cancel()

	runCtx := engine.NewContext(ctx)
	defer runCtx.Close()

	// Initialize DOM Environment
	nm := network.NewNetworkManager()
	dom.SetupDOM(runCtx.VM(), doc, *htmlPath)
	dom.SetupCookies(runCtx.VM(), nm.CookieJar(), *htmlPath)

	// Initialize Network Environment
	if *recordPath != "" {
		nm.Optimizer.SetRecordMode(true)
	}
	nm.SetupNetwork(runCtx)

	if *bootstrapScripts && doc != nil {
		err := bootstrapPageScripts(runCtx, doc, *htmlPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error bootstrapping page scripts: %v\n", err)
			os.Exit(1)
		}
	}

	dom.DispatchLifecycleEvents(runCtx.VM())

	// Execute Script
	runCtx.RunScript(scriptName, scriptContent)

	err := runCtx.EventLoop()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Execution error: %v\n", err)
		os.Exit(1)
	}

	// Save recorded requests if record mode was active
	if *recordPath != "" {
		err := nm.Optimizer.Save(*recordPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error saving recorded requests: %v\n", err)
			os.Exit(1)
		}
	}

	// If print-dom is requested, output the final HTML
	if *printDOM {
		val := runCtx.VM().Get("document")
		if val != nil {
			if docNode, ok := val.Export().(*dom.Node); ok {
				fmt.Println(docNode.GetOuterHTML())
			}
		}
	}

	// If semantic is requested, output the parsed semantic page graph
	if *semanticFlag {
		val := runCtx.VM().Get("document")
		if val != nil {
			if docNode, ok := val.Export().(*dom.Node); ok {
				graph := semantic.ParseSemanticGraph(docNode)
				indentData, err := json.MarshalIndent(graph, "", "  ")
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error serializing semantic graph: %v\n", err)
					os.Exit(1)
				}
				fmt.Println(string(indentData))
			}
		}
	}

	if *statsFlag {
		printStats(startTime, nm)
	}
}

func printStats(startTime time.Time, nm *network.NetworkManager) {
	duration := time.Since(startTime)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Fprintf(os.Stderr, "\n================ CLI PERFORMANCE METRICS ================\n")
	fmt.Fprintf(os.Stderr, "- Execution Time:   %.3f seconds\n", duration.Seconds())
	fmt.Fprintf(os.Stderr, "- Go Heap Alloc:    %.2f MB\n", float64(m.Alloc)/(1024*1024))
	fmt.Fprintf(os.Stderr, "- Go System Sys:    %.2f MB\n", float64(m.Sys)/(1024*1024))
	if nm != nil {
		fmt.Fprintf(os.Stderr, "- Network Requests: %d\n", nm.GetTotalRequests())
	}
	fmt.Fprintf(os.Stderr, "=========================================================\n")
}

func loadHTML(pathOrURL string) (string, error) {
	if strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://") {
		client := &http.Client{Timeout: 15 * time.Second}
		req, err := http.NewRequest("GET", pathOrURL, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")

		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(body), nil
	}

	body, err := os.ReadFile(pathOrURL)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
