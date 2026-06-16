package main

import (
	"context"
	"log"

	"github.com/Ketan6969/capy"
)

func main() {
	// Create a new capy context
	bl := capy.New(context.Background())
	defer bl.Close()

	// Load a live URL. This will fetch the HTML, build the virtual DOM,
	// and execute all synchronous script tags on the page automatically.
	log.Println("Fetching example.com...")
	err := bl.LoadURL("https://example.com")
	if err != nil {
		log.Fatalf("Failed to fetch URL: %v", err)
	}

	// Now run our custom extraction script
	script := `
		console.log("--- Extracted Info ---");
		console.log("Title: " + document.title);
		const links = document.querySelectorAll('a');
		console.log("Found " + links.length + " links.");
		links.forEach(l => console.log(" - " + l.href));
		console.log("----------------------");
	`
	
	err = bl.Evaluate(script)
	if err != nil {
		log.Fatalf("Evaluation failed: %v", err)
	}
}
