package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/Ketan6969/capy"
)

func fetchAndExtract(url string, wg *sync.WaitGroup) {
	defer wg.Done()

	// 1. Create a fresh isolated context for each URL
	// We set a 5-second timeout for the entire operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bl := capy.New(ctx)
	defer bl.Close()

	log.Printf("[%s] Starting fetch...\n", url)
	err := bl.LoadURL(url)
	if err != nil {
		log.Printf("[%s] Error loading URL: %v\n", url, err)
		return
	}

	// 2. Evaluate a quick script
	script := `console.log("Found title: " + document.title);`
	err = bl.Evaluate(script)
	if err != nil {
		log.Printf("[%s] Error evaluating: %v\n", url, err)
		return
	}
	log.Printf("[%s] Finished.\n", url)
}

func main() {
	// Because capy uses Goja (which is incredibly light and memory safe),
	// we can easily spin up dozens of isolated runtimes concurrently!
	urls := []string{
		"https://example.com",
		"https://httpbin.org/html",
		"https://news.ycombinator.com",
	}

	var wg sync.WaitGroup
	for _, u := range urls {
		wg.Add(1)
		go fetchAndExtract(u, &wg)
	}

	wg.Wait()
	log.Println("All concurrent scrapes completed!")
}
