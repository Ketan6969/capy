package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type ExtractedData struct {
	Title      string   `json:"title"`
	DomNodes   int      `json:"domNodes"`
	Images     int      `json:"images"`
	Buttons    int      `json:"buttons"`
	Forms      int      `json:"forms"`
	H1Count    int      `json:"h1Count"`
	H2Count    int      `json:"h2Count"`
	Links      []string `json:"links"`
	TextLength int      `json:"textLength"`
	WordCount  int      `json:"wordCount"`
	Hydration  struct {
		Root bool `json:"root"`
		Next bool `json:"next"`
		App  bool `json:"app"`
	} `json:"hydration"`
}

type Result struct {
	Time        string
	Memory      string
	DataOut     string
	NetRequests int
}

func minScore(a, b int) float64 {
	if a == 0 && b == 0 {
		return 100.0
	}
	if a == 0 || b == 0 {
		return 0.0
	}
	if a > b {
		return float64(b) / float64(a) * 100.0
	}
	return float64(a) / float64(b) * 100.0
}

func main() {
	fmt.Println("==================================================================")
	fmt.Println("   BROWSERLESS RUNTIME BENCHMARK V2")
	fmt.Println("==================================================================")

	cmdBuild := exec.Command("go", "build", "-o", "browserless", "./cmd/browserless")
	cmdBuild.Run()

	data, err := os.ReadFile("tests/corpus.json")
	if err != nil {
		fmt.Println("Error reading corpus:", err)
		return
	}

	var targets []string
	json.Unmarshal(data, &targets)

	fmt.Printf("| %-25s | %-10s | %-10s | %-10s | %-10s | %-10s | %-10s | %-10s |\n", "Target", "Title", "DOM", "Content", "Links", "Network", "Hydration", "OVERALL")
	fmt.Println("|---------------------------|------------|------------|------------|------------|------------|------------|------------|")

	type benchRes struct {
		URL string
		TitleScore, DomScore, ContentScore, LinkScore, NetworkScore, HydrationScore, Overall float64
	}

	resultsChan := make(chan benchRes, len(targets))
	jobs := make(chan string, len(targets))

	for w := 1; w <= 10; w++ {
		go func() {
			for url := range jobs {
				pwRes := runCommand("node", "tests/playwright_runner.js", url, "tests/extract_v2.js")
				blRes := runCommand("./browserless", "-timeout", "15", "-stats", "-html", url, "-file", "tests/extract_v2.js")

				var pwData, blData ExtractedData
				json.Unmarshal([]byte(pwRes.DataOut), &pwData)
				json.Unmarshal([]byte(blRes.DataOut), &blData)

				titleScore := 0.0
				if pwData.Title == blData.Title && pwData.Title != "" {
					titleScore = 100.0
				} else if strings.Contains(pwData.Title, blData.Title) || strings.Contains(blData.Title, pwData.Title) {
					titleScore = 80.0
				}

				domScore := minScore(pwData.DomNodes, blData.DomNodes)
				contentScore := minScore(pwData.WordCount, blData.WordCount)
				networkScore := minScore(pwRes.NetRequests, blRes.NetRequests)

				pwLinks := make(map[string]bool)
				for _, l := range pwData.Links { pwLinks[l] = true }
				blLinks := make(map[string]bool)
				for _, l := range blData.Links { blLinks[l] = true }
				
				overlap := 0
				for l := range blLinks { if pwLinks[l] { overlap++ } }
				linkScore := 100.0
				if len(pwLinks) > 0 { linkScore = float64(overlap) / float64(len(pwLinks)) * 100.0 }

				hydrationScore := 100.0
				pwHyd := 0
				blHyd := 0
				if pwData.Hydration.Root { pwHyd++ }
				if pwData.Hydration.Next { pwHyd++ }
				if pwData.Hydration.App { pwHyd++ }
				if blData.Hydration.Root { blHyd++ }
				if blData.Hydration.Next { blHyd++ }
				if blData.Hydration.App { blHyd++ }
				
				if pwHyd > 0 && blHyd == 0 { hydrationScore = 0.0 }

				overall := (titleScore * 0.20) + (domScore * 0.20) + (contentScore * 0.25) + (linkScore * 0.15) + (networkScore * 0.10) + (hydrationScore * 0.10)
				
				resultsChan <- benchRes{
					URL: url, TitleScore: titleScore, DomScore: domScore, ContentScore: contentScore,
					LinkScore: linkScore, NetworkScore: networkScore, HydrationScore: hydrationScore, Overall: overall,
				}
			}
		}()
	}

	for _, url := range targets {
		jobs <- url
	}
	close(jobs)

	var totalScore float64
	for i := 0; i < len(targets); i++ {
		res := <-resultsChan
		totalScore += res.Overall
		fmt.Printf("| %-25s | %5.0f%%     | %5.0f%%     | %5.0f%%     | %5.0f%%     | %5.0f%%     | %5.0f%%     | **%5.0f%%** |\n", 
			urlShort(res.URL), res.TitleScore, res.DomScore, res.ContentScore, res.LinkScore, res.NetworkScore, res.HydrationScore, res.Overall)
	}

	avgScore := totalScore / float64(len(targets))
	fmt.Println()
	fmt.Printf("=> **AVERAGE OVERALL MATCH:** %.1f%%\n", avgScore)
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

	// Parse Time
	reTime := regexp.MustCompile(`Elapsed \(wall clock\) time \(h:mm:ss or m:ss\):\s+([0-9:.]+)`)
	tm := reTime.FindStringSubmatch(errStr)
	if len(tm) > 1 {
		timeStr := tm[1]
		parts := strings.Split(timeStr, ":")
		totalSecs := 0.0
		if len(parts) == 2 {
			var mi int
			var s float64
			fmt.Sscanf(parts[0], "%d", &mi)
			fmt.Sscanf(parts[1], "%f", &s)
			totalSecs = float64(mi)*60.0 + s
		} else if len(parts) == 3 {
			var h, mi int
			var s float64
			fmt.Sscanf(parts[0], "%d", &h)
			fmt.Sscanf(parts[1], "%d", &mi)
			fmt.Sscanf(parts[2], "%f", &s)
			totalSecs = float64(h)*3600.0 + float64(mi)*60.0 + s
		}
		res.Time = fmt.Sprintf("%.2f", totalSecs)
	}

	fullOut := outStr + "\n" + errStr
	lines := strings.Split(fullOut, "\n")
	for _, l := range lines {
		if strings.Contains(l, "OUT::") {
			parts := strings.SplitN(l, "OUT::", 2)
			if len(parts) > 1 {
				res.DataOut = strings.TrimSpace(parts[1])
			}
		}
		if strings.Contains(l, "NET_REQ::") {
			parts := strings.SplitN(l, "NET_REQ::", 2)
			if len(parts) > 1 {
				fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &res.NetRequests)
			}
		}
		if strings.Contains(l, "- Network Requests:") {
			parts := strings.Split(l, ":")
			if len(parts) > 1 {
				fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &res.NetRequests)
			}
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
