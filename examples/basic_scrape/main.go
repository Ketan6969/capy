package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Ketan6969/capy"
)

func main() {
	// 1. Create a new Capy instance
	bl := capy.New(context.Background())
	defer bl.Close()

	// 2. Load some raw HTML directly
	html := `
	<html>
		<head><title>My Example Site</title></head>
		<body>
			<div class="pricing">
				<h2>Pro Plan</h2>
				<p>$20 / month</p>
			</div>
		</body>
	</html>
	`
	err := bl.LoadHTML(html)
	if err != nil {
		log.Fatalf("Failed to load HTML: %v", err)
	}

	// 3. Evaluate a snippet of JS to extract data from the DOM
	script := `
		const title = document.querySelector('title').innerText;
		const priceNode = document.querySelector('.pricing p');
		
		// Return JSON string from script execution
		JSON.stringify({
			title: title,
			price: priceNode ? priceNode.innerText : "Not found"
		});
	`
	
	// Print extraction
	fmt.Println("Extracting data via JS evaluation...")
	err = bl.Evaluate("const result = " + script + "; console.log('Extracted:', result);")
	if err != nil {
		log.Fatal(err)
	}
}
