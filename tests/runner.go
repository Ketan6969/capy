//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
)

type TestResults struct {
	Suite         string
	PageLoaded    bool
	JSExecuted    bool
	DOMUpdated    bool
	FetchWorked   bool
	TimersWorked  bool
	EventsWorked  bool
	StorageWorked bool
	TimeSec       string
	HeapAlloc     string
	SysMem        string
}

func main() {
	suites := []string{
		"simple-html",
		"react",
		"vue",
		"nextjs",
		"shopify",
		"maps",
		"reddit",
		"github",
	}

	fmt.Println("======================================================================")
	fmt.Println("              BROWSERLESS RUNTIME BENCHMARK SUITE                     ")
	fmt.Println("======================================================================")
	fmt.Printf("Running %d framework/site test suites...\n\n", len(suites))

	results := []TestResults{}

	// Compile regexp patterns
	resultsPatterns := map[string]*regexp.Regexp{
		"pageLoaded":    regexp.MustCompile(`\[RESULT\] Page Loaded:\s*(PASS|FAIL)`),
		"jsExecuted":    regexp.MustCompile(`\[RESULT\] JS Executed:\s*(PASS|FAIL)`),
		"domUpdated":    regexp.MustCompile(`\[RESULT\] DOM Updated:\s*(PASS|FAIL)`),
		"fetchWorked":   regexp.MustCompile(`\[RESULT\] Fetch Worked:\s*(PASS|FAIL)`),
		"timersWorked":  regexp.MustCompile(`\[RESULT\] Timers Worked:\s*(PASS|FAIL)`),
		"eventsWorked":  regexp.MustCompile(`\[RESULT\] Events Worked:\s*(PASS|FAIL)`),
		"storageWorked": regexp.MustCompile(`\[RESULT\] Storage Worked:\s*(PASS|FAIL)`),
	}

	timePattern := regexp.MustCompile(`-\s*Execution Time:\s*([\d.]+)\s*seconds`)
	heapPattern := regexp.MustCompile(`-\s*Go Heap Alloc:\s*([\d.]+)\s*MB`)
	sysPattern := regexp.MustCompile(`-\s*Go System Sys:\s*([\d.]+)\s*MB`)

	for _, suite := range suites {
		fmt.Printf("Executing suite [%s]... ", suite)

		htmlPath := fmt.Sprintf("tests/engine-smoke/%s/index.html", suite)
		scriptPath := fmt.Sprintf("tests/engine-smoke/%s/script.js", suite)

		cmd := exec.Command("./browserless", "-html", htmlPath, "-file", scriptPath, "-stats")
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err != nil {
			fmt.Printf("FAILED to run command: %v\n", err)
			continue
		}
		fmt.Println("DONE")

		stdoutStr := stdout.String()
		stderrStr := stderr.String()

		res := TestResults{
			Suite:   suite,
			TimeSec: "N/A",
		}

		// Parse results from stdout
		res.PageLoaded = parsePassFail(resultsPatterns["pageLoaded"].FindStringSubmatch(stdoutStr))
		res.JSExecuted = parsePassFail(resultsPatterns["jsExecuted"].FindStringSubmatch(stdoutStr))
		res.DOMUpdated = parsePassFail(resultsPatterns["domUpdated"].FindStringSubmatch(stdoutStr))
		res.FetchWorked = parsePassFail(resultsPatterns["fetchWorked"].FindStringSubmatch(stdoutStr))
		res.TimersWorked = parsePassFail(resultsPatterns["timersWorked"].FindStringSubmatch(stdoutStr))
		res.EventsWorked = parsePassFail(resultsPatterns["eventsWorked"].FindStringSubmatch(stdoutStr))
		res.StorageWorked = parsePassFail(resultsPatterns["storageWorked"].FindStringSubmatch(stdoutStr))

		// Parse performance metrics from stderr
		if m := timePattern.FindStringSubmatch(stderrStr); len(m) > 1 {
			res.TimeSec = m[1]
		}
		if m := heapPattern.FindStringSubmatch(stderrStr); len(m) > 1 {
			res.HeapAlloc = m[1]
		}
		if m := sysPattern.FindStringSubmatch(stderrStr); len(m) > 1 {
			res.SysMem = m[1]
		}

		results = append(results, res)
	}

	// Print beautiful markdown-like ASCII grid
	fmt.Println("\n==================================== BENCHMARK RESULTS ====================================")
	fmt.Println()
	fmt.Printf("| %-12s | %-6s | %-6s | %-6s | %-6s | %-6s | %-6s | %-6s | %-8s | %-9s | %-8s |\n",
		"Suite", "Page", "JS", "DOM", "Fetch", "Timer", "Event", "Store", "Time(s)", "Heap(MB)", "Sys(MB)")
	fmt.Println("|--------------|--------|--------|--------|--------|--------|--------|--------|----------|-----------|----------|")

	for _, r := range results {
		fmt.Printf("| %-12s | %-6s | %-6s | %-6s | %-6s | %-6s | %-6s | %-6s | %-8s | %-9s | %-8s |\n",
			r.Suite,
			check(r.PageLoaded),
			check(r.JSExecuted),
			check(r.DOMUpdated),
			check(r.FetchWorked),
			check(r.TimersWorked),
			check(r.EventsWorked),
			check(r.StorageWorked),
			r.TimeSec,
			r.HeapAlloc,
			r.SysMem,
		)
	}
	fmt.Println("===========================================================================================")
	fmt.Println("Note: Time(s) measures total Go CLI process execution time. Memory values represent Go MemStats.")
}

func parsePassFail(m []string) bool {
	if len(m) > 1 && m[1] == "PASS" {
		return true
	}
	return false
}

func check(passed bool) string {
	if passed {
		// Use green checkmark in supporting terminals
		return "\033[32m✓\033[0m"
	}
	return "\033[31m✗\033[0m"
}
