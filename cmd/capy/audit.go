package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"strings"
	"time"

	"github.com/Ketan6969/capy/internal/dom"
	"github.com/Ketan6969/capy/internal/engine"
	"github.com/Ketan6969/capy/internal/network"
)

// AuditResult holds all metrics collected during an audit.
type AuditResult struct {
	PageLoaded         bool
	MissingAPIs        []string
	UsedAPIs           []string
	NetworkRequests    int
	DOMNodes           int
	JSErrors           []string
	CompatibilityScore int
}

// apiDef describes a browser API: its global JS name and display name.
type apiDef struct {
	jsName  string // global variable name in our runtime
	display string // shown in audit output
}

// knownAPIs is the canonical list of APIs we track.
var knownAPIs = []apiDef{
	{jsName: "ResizeObserver", display: "ResizeObserver"},
	{jsName: "IntersectionObserver", display: "IntersectionObserver"},
	{jsName: "MutationObserver", display: "MutationObserver"},
	{jsName: "PerformanceObserver", display: "PerformanceObserver"},
	{jsName: "Worker", display: "WebWorker"},
	{jsName: "ServiceWorker", display: "ServiceWorker"},
	{jsName: "WebSocket", display: "WebSocket"},
	{jsName: "crypto", display: "WebCrypto"},
	{jsName: "indexedDB", display: "IndexedDB"},
	{jsName: "requestAnimationFrame", display: "requestAnimationFrame"},
	{jsName: "Promise", display: "Promise"},
	{jsName: "fetch", display: "fetch"},
	{jsName: "localStorage", display: "localStorage"},
	{jsName: "sessionStorage", display: "sessionStorage"},
	{jsName: "CustomEvent", display: "CustomEvent"},
	{jsName: "history", display: "History API"},
	{jsName: "navigator", display: "Navigator"},
	{jsName: "location", display: "Location API"},
}

// countDOMNodes recursively counts all nodes in the DOM tree.
func countDOMNodes(node *dom.Node) int {
	if node == nil {
		return 0
	}
	count := 1
	for _, child := range node.ChildNodes {
		count += countDOMNodes(child)
	}
	return count
}

// runAudit performs a full audit of the given URL and prints the results.
func runAudit(targetURL string, timeout time.Duration) {
	startTime := time.Now()

	fmt.Printf("\033[2m  ⏳ Fetching and executing %s...\033[0m", targetURL)

	// ── 1. Load HTML Content ──────────────────────────────────────────────────
	htmlContent, err := loadHTML(targetURL)
	pageLoaded := err == nil && strings.TrimSpace(htmlContent) != ""
	if err != nil {
		slog.Error("Failed to load HTML", "url", targetURL, "error", err)
		fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", targetURL, err)
		os.Exit(1)
	}
	slog.Debug("HTML loaded", "bytes", len(htmlContent))

	// ── 2. Initialize Runtime and Context ─────────────────────────────────────
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	runCtx := engine.NewContext(ctx)
	defer runCtx.Close()

	vm := runCtx.VM()

	// Expose __audit_log__ to JS — always DEBUG so nothing leaks to stderr during normal runs.
	// Errors are already captured in jsErrors and shown in the final formatted report.
	vm.Set("__audit_log__", func(level string, msg string) {
		slog.Debug("audit-js [" + level + "] " + msg)
	})

	var doc *dom.Node
	if pageLoaded {
		doc, _ = dom.ParseHTML(htmlContent)
	}
	dom.SetupDOM(runCtx.VM(), doc, targetURL)

	// In audit mode, silence console.log/warn/info — third-party libraries (jQuery Migrate,
	// React, etc.) print banners we don't want mixed into audit output.
	// console.error is redirected to the error collector so real errors are still captured.
	silenceScript := `(function() {
		const _noop = function() {};
		console.log   = _noop;
		console.warn  = _noop;
		console.info  = _noop;
		console.debug = _noop;
		console.error = function() {
			const parts = [];
			for (let i = 0; i < arguments.length; i++) {
				try { parts.push(String(arguments[i])); } catch(e) { parts.push('[unencodable]'); }
			}
			if (globalThis.__audit_log__) globalThis.__audit_log__('error', parts.join(' '));
		};
	})();`
	if _, err := vm.RunString(silenceScript); err != nil {
		slog.Debug("console silence script failed", "err", err)
	}

	// ── 3. Set up instrumented network (count fetch calls) ───────────────────
	nm := network.NewNetworkManager()
	networkCount := 0
	dom.SetupCookies(runCtx.VM(), nm.CookieJar(), targetURL)
	nm.SetupNetworkInstrumented(runCtx, &networkCount)

	// ── 4. Collect JS errors ─────────────────────────────────────────────────
	var jsErrors []string
	runCtx.SetErrorCollector(func(msg string) {
		jsErrors = append(jsErrors, msg)
	})

	// ── 5. Inject API Tracker ────────────────────────────────────────────────
	var apiNames []string
	for _, api := range knownAPIs {
		apiNames = append(apiNames, api.jsName)
	}
	vm.Set("__audit_known_apis__", apiNames)

	trackerScript := `
	globalThis.__audit_used_apis__ = new Set();
	globalThis.__audit_missing_apis__ = new Set();
	(function() {
		for (const api of globalThis.__audit_known_apis__) {
			let original = undefined;
			let isDefined = false;
			try {
				if (api in globalThis && globalThis[api] !== undefined) {
					original = globalThis[api];
					isDefined = true;
				}
			} catch(e) {}

			// Set a getter to trap access
			try {
				Object.defineProperty(globalThis, api, {
					get: function() {
						globalThis.__audit_used_apis__.add(api);
						if (!isDefined) {
							globalThis.__audit_missing_apis__.add(api);
						}
						return original;
					},
					set: function(v) {
						globalThis.__audit_used_apis__.add(api);
						if (!isDefined) {
							globalThis.__audit_missing_apis__.add(api);
						}
						original = v;
						isDefined = true;
					},
					configurable: true,
					enumerable: true
				});
			} catch(e) {}
		}
	})();
	`
	runCtx.RunScript("audit-tracker", trackerScript)

	vm.Set("__audit_base_url__", targetURL)

	loaderScript := `
	(async function() {
		if (typeof document === 'undefined') {
			if (globalThis.__audit_log__) globalThis.__audit_log__('error', "document is undefined!");
			return;
		}
		globalThis.__audit_loader_errors__ = [];
		const scripts = document.getElementsByTagName('script');
		if (globalThis.__audit_log__) globalThis.__audit_log__('debug', "Found script tags: " + (scripts ? scripts.length : 0));
		
		// Copy scripts into an array to avoid live HTMLCollection mutation issues
		const scriptList = [];
		for (let i = 0; i < scripts.length; i++) {
			scriptList.push(scripts[i]);
		}
		
		// Helper: strip CDATA wrappers that WordPress injects for XML compatibility.
		// e.g. /* <![CDATA[ */ ... /* ]]> */  or  //<![CDATA[  ...  //]]>
		function stripCDATA(text) {
			return text
				.replace(/\/\*\s*<!\[CDATA\[\s*\*\//g, '')
				.replace(/\/\*\s*\]\]>\s*\*\//g, '')
				.replace(/\/\/<!\[CDATA\[/g, '')
				.replace(/\/\/\]\]>/g, '');
		}

		const fetchPromises = scriptList
			.filter(s => {
				// Only process executable JS script tags. Skip JSON-LD, Handlebars,
				// templates, and any other non-JS payload that would fail to eval().
				const type = (s.getAttribute('type') || '').toLowerCase().trim();
				return type === '' ||
					type === 'text/javascript' ||
					type === 'application/javascript' ||
					type === 'text/ecmascript' ||
					type === 'module';
			})
			.map(s => {
			let src = s.getAttribute('src');
			if (src) {
				if (!src.startsWith('http://') && !src.startsWith('https://')) {
					let base = globalThis.__audit_base_url__ || "";
					if (base.startsWith('http')) {
						if (src.startsWith('//')) {
							src = base.substring(0, base.indexOf('//')) + src;
						} else if (src.startsWith('/')) {
							const m = base.match(/^(https?:\/\/[^\/]+)/);
							if (m) src = m[1] + src;
						} else {
							const lastSlash = base.lastIndexOf('/');
							if (lastSlash > 8) {
								src = base.substring(0, lastSlash + 1) + src;
							} else {
								src = base + '/' + src;
							}
						}
					}
				}
				if (globalThis.__audit_log__) globalThis.__audit_log__('debug', "initiating fetch for " + src);
				return fetch(src).then(res => res.text()).then(text => ({ type: 'remote', src: src, text: stripCDATA(text) })).catch(e => ({ type: 'error', src: src, error: e }));
			} else {
				return Promise.resolve({ type: 'inline', text: stripCDATA(s.textContent) });
			}
		});

		const scriptsData = await Promise.all(fetchPromises);

		for (const sc of scriptsData) {
			if (sc.type === 'error') {
				if (globalThis.__audit_log__) globalThis.__audit_log__('error', "error fetching " + sc.src + ": " + sc.error.toString());
				globalThis.__audit_loader_errors__.push(sc.error.toString());
				continue;
			}
			if (sc.type === 'remote') {
				try {
					if (globalThis.__audit_log__) globalThis.__audit_log__('debug', "evaluating remote script " + sc.src);
					(0, eval)(sc.text);
				} catch(e) {
					if (globalThis.__audit_log__) globalThis.__audit_log__('error', "error processing remote script " + sc.src + ": " + e.toString());
					globalThis.__audit_loader_errors__.push(e.toString());
				}
			} else if (sc.type === 'inline' && sc.text && sc.text.trim() !== '') {
				try {
					if (globalThis.__audit_log__) globalThis.__audit_log__('debug', "evaluating inline script");
					(0, eval)(sc.text);
				} catch(e) {
					if (globalThis.__audit_log__) globalThis.__audit_log__('error', "error processing inline script: " + e.toString());
					globalThis.__audit_loader_errors__.push(e.toString());
				}
			}
		}
		if (globalThis.__audit_done__) globalThis.__audit_done__();
	})().catch(e => { 
		if (globalThis.__audit_log__) globalThis.__audit_log__('error', "unhandled loader error: " + e.toString());
		globalThis.__audit_loader_errors__.push(e.toString()); 
		if (globalThis.__audit_done__) globalThis.__audit_done__();
	});
	`
	vm.Set("__audit_done__", func() {
		runCtx.Cancel()
	})
	runCtx.RunScript("audit-loader", loaderScript)

	// Run a lightweight probe that exercises storage + promises just in case
	probeScript := `(function() {
  try { localStorage.setItem('__audit__', '1'); localStorage.removeItem('__audit__'); } catch(e) {}
  try { sessionStorage.setItem('__audit__', '1'); sessionStorage.removeItem('__audit__'); } catch(e) {}
  try { Promise.resolve(42).then(function(v) {}); } catch(e) {}
})();`
	runCtx.RunScript("audit-probe", probeScript)

	// Wait for scripts to fetch and evaluate
	errLoop := runCtx.EventLoop()
	if errLoop != nil && errLoop != context.Canceled && errLoop.Error() != "context canceled" {
		jsErrors = append(jsErrors, errLoop.Error())
	}

	// ── 7. Check which of the page's used APIs our runtime provides ───────────
	getArray := func(name string) []string {
		arrVal, _ := vm.RunString(fmt.Sprintf(`Array.from(globalThis.%s || [])`, name))
		var res []string
		if arrVal != nil {
			if exported, ok := arrVal.Export().([]interface{}); ok {
				for _, v := range exported {
					if s, ok := v.(string); ok {
						res = append(res, s)
					}
				}
			}
		}
		return res
	}

	jsErrors = append(jsErrors, getArray("__audit_loader_errors__")...)

	usedSet := make(map[string]bool)
	for _, api := range getArray("__audit_used_apis__") {
		usedSet[api] = true
	}

	missingSet := make(map[string]bool)
	for _, api := range getArray("__audit_missing_apis__") {
		missingSet[api] = true
	}

	var missingAPIs []string
	var presentAPINames []string
	for _, api := range knownAPIs {
		if usedSet[api.jsName] {
			if missingSet[api.jsName] {
				missingAPIs = append(missingAPIs, api.display)
			} else {
				presentAPINames = append(presentAPINames, api.display)
			}
		}
	}

	// DOM node count
	domNodes := 0
	if doc != nil {
		domNodes = countDOMNodes(doc)
	}

	// ── 8. Compute per-site compatibility score ───────────────────────────────
	compatScore := 100
	totalUsed := len(presentAPINames) + len(missingAPIs)
	if totalUsed == 0 {
		// No detectable API usage — page is probably static HTML, full score
		compatScore = 100
	} else {
		presentCount := len(presentAPINames)
		compatScore = int(math.Round(float64(presentCount) / float64(totalUsed) * 100))
	}

	// Deduct for JS errors (capped at -15 pts, 3 pts each)
	errorPenalty := len(jsErrors) * 3
	if errorPenalty > 15 {
		errorPenalty = 15
	}
	compatScore -= errorPenalty
	if compatScore < 0 {
		compatScore = 0
	}

	result := AuditResult{
		PageLoaded:         pageLoaded,
		MissingAPIs:        missingAPIs,
		UsedAPIs:           presentAPINames,
		NetworkRequests:    networkCount,
		DOMNodes:           domNodes,
		JSErrors:           jsErrors,
		CompatibilityScore: compatScore,
	}

	elapsed := time.Since(startTime)
	fmt.Print("\r\033[K") // clear the spinner line
	printAuditResult(targetURL, result, elapsed)
}

// printAuditResult pretty-prints the audit result to stdout.
func printAuditResult(url string, r AuditResult, elapsed time.Duration) {
	green := "\033[32m"
	red := "\033[31m"
	yellow := "\033[33m"
	bold := "\033[1m"
	reset := "\033[0m"
	cyan := "\033[36m"
	dim := "\033[2m"

	bar := strings.Repeat("─", 52)

	fmt.Printf("\n%s%s%s\n", bold, bar, reset)
	fmt.Printf("%s  capy audit%s\n", bold, reset)
	fmt.Printf("%s  %s%s\n", dim, url, reset)
	fmt.Printf("%s%s%s\n\n", bold, bar, reset)

	// Page Loaded
	if r.PageLoaded {
		fmt.Printf("  %-24s %s✓%s\n", "Page Loaded:", green, reset)
	} else {
		fmt.Printf("  %-24s %s✗%s\n", "Page Loaded:", red, reset)
	}
	fmt.Println()

	// Missing APIs
	fmt.Printf("  %sMissing APIs:%s\n", bold, reset)
	if len(r.MissingAPIs) == 0 {
		fmt.Printf("    %s(none — full coverage for this page)%s\n", green, reset)
	} else {
		for _, api := range r.MissingAPIs {
			fmt.Printf("    %s- %s%s\n", yellow, api, reset)
		}
	}
	fmt.Println()

	// Detected APIs (present)
	if len(r.UsedAPIs) > 0 {
		fmt.Printf("  %sAPIs Supported:%s\n", bold, reset)
		for _, api := range r.UsedAPIs {
			fmt.Printf("    %s✓ %s%s\n", green, api, reset)
		}
		fmt.Println()
	}

	// Network Requests
	fmt.Printf("  %-24s %s%d%s\n", "Network Requests:", cyan, r.NetworkRequests, reset)
	fmt.Println()

	// DOM Nodes
	fmt.Printf("  %-24s %s%d%s\n", "DOM Nodes:", cyan, r.DOMNodes, reset)
	fmt.Println()

	// JS Errors
	errColor := green
	if len(r.JSErrors) > 0 {
		errColor = red
	}
	fmt.Printf("  %-24s %s%d%s\n", "JS Errors:", errColor, len(r.JSErrors), reset)
	if len(r.JSErrors) > 0 {
		// Deduplicate errors — truncate to first ~60 chars as the key
		type errEntry struct {
			msg   string
			count int
		}
		seen := make(map[string]*errEntry)
		var order []string
		for _, e := range r.JSErrors {
			key := e
			if len(key) > 60 {
				key = key[:60]
			}
			if entry, ok := seen[key]; ok {
				entry.count++
			} else {
				seen[key] = &errEntry{msg: e, count: 1}
				order = append(order, key)
			}
		}
		shown := 0
		for _, key := range order {
			if shown >= 8 {
				fmt.Printf("    %s• … and %d more%s\n", red, len(r.JSErrors)-shown, reset)
				break
			}
			entry := seen[key]
			msg := entry.msg
			if len(msg) > 72 {
				msg = msg[:69] + "..."
			}
			if entry.count > 1 {
				fmt.Printf("    %s• %s (×%d)%s\n", red, msg, entry.count, reset)
			} else {
				fmt.Printf("    %s• %s%s\n", red, msg, reset)
			}
			shown++
		}
	}
	fmt.Println()

	// Compatibility Score
	scoreColor := green
	switch {
	case r.CompatibilityScore < 50:
		scoreColor = red
	case r.CompatibilityScore < 75:
		scoreColor = yellow
	}

	bar2 := buildScoreBar(r.CompatibilityScore, 20)
	fmt.Printf("  %sCompatibility Score:%s  %s%d%%%s\n", bold, reset, scoreColor, r.CompatibilityScore, reset)
	fmt.Printf("  [%s%s%s]\n", scoreColor, bar2, reset)
	fmt.Println()

	fmt.Printf("%s%s%s\n", dim, bar, reset)
	fmt.Printf("%s  Audited in %.2fs%s\n\n", dim, elapsed.Seconds(), reset)
}

// buildScoreBar renders a simple ASCII progress bar.
func buildScoreBar(score, width int) string {
	filled := int(math.Round(float64(score) / 100.0 * float64(width)))
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}
