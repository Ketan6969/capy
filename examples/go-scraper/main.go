package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Ketan6969/capy"
)

func main() {
	scrapeExampleCom()
	fmt.Println("\n------------------------------------------------")
	scrapeQuotes()
}

func scrapeExampleCom() {
	fmt.Println("=== Scraping Example.com ===")
	
	// 1. Initialize capy
	bl := capy.New(context.Background())
	defer bl.Close()

	// 2. Load the page natively (no headless browser overhead!)
	if err := bl.LoadURL("https://example.com"); err != nil {
		log.Fatal(err)
	}

	// 3. Access the Native Go DOM
	doc := bl.Document()
	
	// 4. Query elements using Go
	titleNode := doc.QuerySelector("h1")
	if titleNode != nil {
		fmt.Printf("Title: %s\n", titleNode.GetInnerText())
	}

	paragraphNode := doc.QuerySelector("p")
	if paragraphNode != nil {
		fmt.Printf("Description: %s\n", paragraphNode.GetInnerText())
	}
}

func scrapeQuotes() {
	fmt.Println("=== Scraping Quotes To Scrape ===")
	
	bl := capy.New(context.Background())
	defer bl.Close()

	if err := bl.LoadURL("http://quotes.toscrape.com"); err != nil {
		log.Fatal(err)
	}

	doc := bl.Document()
	quotes := doc.QuerySelectorAll(".quote")

	// Limit to the first 3 quotes
	for i := 0; i < len(quotes) && i < 3; i++ {
		q := quotes[i]
		
		textNode := q.QuerySelector(".text")
		authorNode := q.QuerySelector(".author")

		if textNode != nil && authorNode != nil {
			fmt.Printf("Quote %d: %s\n", i+1, textNode.GetInnerText())
			fmt.Printf("Author: %s\n\n", authorNode.GetInnerText())
		}
	}
}
