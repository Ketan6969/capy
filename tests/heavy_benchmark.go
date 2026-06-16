//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var targets = []string{
	"https://news.ycombinator.com",
	"https://reddit.com",
	"https://github.com",
	"https://nike.com",
	"https://bulletproof.com", // Shopify
	"https://vercel.com",      // Next.js
	"https://react.dev",       // React
	"https://vuejs.org",       // Vue
	"https://angular.dev",     // Angular
}

type Result struct {
	Time    string
	Memory  string
	DataOut string
}

func main() {

	fmt.Println("==================================================================")
	fmt.Println("   PLAYWRIGHT VS BROWSERLESS HEAVY BENCHMARK")
	fmt.Println("==================================================================")

	// Pre-build capy
	cmdBuild := exec.Command("go", "build", "-o", "capy", "./cmd/capy")
	cmdBuild.Run()

	fmt.Printf("| %-25s | %-20s | %-20s | %-12s |\n", "Target", "Playwright (Time/RAM)", "Capy (Time/RAM)", "Data Match")
	fmt.Println("|---------------------------|----------------------|----------------------|--------------|")

	for _, url := range targets {
		pwRes := runCommand("node", "tests/playwright_runner.js", url, "tests/extract.js")
		blRes := runCommand("./capy", "-timeout", "15", "-html", url, "-file", "tests/extract.js")

		match := "❌ No"
		if pwRes.DataOut != "" && blRes.DataOut != "" {
			if pwRes.DataOut == blRes.DataOut {
				match = "✅ Yes"
			} else {
				match = "⚠️ Partial"
			}
		}

		pwDisplay := fmt.Sprintf("%ss / %sMB", pwRes.Time, pwRes.Memory)
		blDisplay := fmt.Sprintf("%ss / %sMB", blRes.Time, blRes.Memory)

		fmt.Printf("| %-25s | %-20s | %-20s | %-12s |\n", urlShort(url), pwDisplay, blDisplay, match)
	}
}

func runCommand(binary string, args ...string) Result {
	cmdArgs := append([]string{"-v", binary}, args...)
	cmd := exec.Command("/usr/bin/time", cmdArgs...)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	cmd.Run()

	outStr := out.String()
	errStr := stderr.String()

	res := Result{
		Time:   "err",
		Memory: "err",
	}

	// Parse RAM (Maximum resident set size in kbytes)
	reMem := regexp.MustCompile(`Maximum resident set size \(kbytes\):\s+(\d+)`)
	m := reMem.FindStringSubmatch(errStr)
	if len(m) > 1 {
		kb := 0
		fmt.Sscanf(m[1], "%d", &kb)
		res.Memory = fmt.Sprintf("%.1f", float64(kb)/1024.0)
	}

	// Parse Time (Elapsed (wall clock) time (h:mm:ss or m:ss))
	reTime := regexp.MustCompile(`Elapsed \(wall clock\) time \(h:mm:ss or m:ss\):\s+([0-9:.]+)`)
	tm := reTime.FindStringSubmatch(errStr)
	if len(tm) > 1 {
		timeStr := tm[1]
		parts := strings.Split(timeStr, ":")
		totalSecs := 0.0
		if len(parts) == 2 {
			var m int
			var s float64
			fmt.Sscanf(parts[0], "%d", &m)
			fmt.Sscanf(parts[1], "%f", &s)
			totalSecs = float64(m)*60.0 + s
		} else if len(parts) == 3 {
			var h, m int
			var s float64
			fmt.Sscanf(parts[0], "%d", &h)
			fmt.Sscanf(parts[1], "%d", &m)
			fmt.Sscanf(parts[2], "%f", &s)
			totalSecs = float64(h)*3600.0 + float64(m)*60.0 + s
		}
		res.Time = fmt.Sprintf("%.2f", totalSecs)
	}

	// Find OUT:: string in either stdout or stderr
	fullOut := outStr + "\n" + errStr
	lines := strings.Split(fullOut, "\n")
	for _, l := range lines {
		if strings.Contains(l, "OUT::") {
			res.DataOut = strings.TrimSpace(strings.SplitN(l, "OUT::", 2)[1])
		}
	}

	return res
}

func urlShort(u string) string {
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "www.")
	if len(u) > 25 {
		return u[:22] + "..."
	}
	return u
}
