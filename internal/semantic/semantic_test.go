package semantic

import (
	"testing"

	"github.com/Ketan6969/capy/internal/dom"
)

func TestParseSemanticGraph(t *testing.T) {
	// 1. Create a dummy page containing a Form, a Table, a List, and a Product Card grid
	htmlContent := `
		<html>
		<body>
			<form action="/login" method="POST">
				<input name="username" type="text" />
				<input name="password" type="password" />
				<button type="submit">Login</button>
			</form>

			<table id="data-table">
				<tr><th>Name</th><th>Age</th></tr>
				<tr><td>Alice</td><td>30</td></tr>
				<tr><td>Bob</td><td>25</td></tr>
			</table>

			<ul class="nav">
				<li>Home</li>
				<li>About</li>
			</ul>

			<div class="products-grid">
				<div class="product-item">
					<img src="/images/shoes.png" alt="Shoes" />
					<h3>Super Running Shoes</h3>
					<span class="price">$120.00</span>
					<a href="/products/shoes" class="buy-btn">Add to Cart</a>
				</div>
			</div>
		</body>
		</html>
	`

	doc, err := dom.ParseHTML(htmlContent)
	if err != nil {
		t.Fatalf("ParseHTML failed: %v", err)
	}

	graph := ParseSemanticGraph(doc)
	if graph == nil {
		t.Fatal("Semantic graph was nil")
	}

	// 2. Validate structures recursively
	var foundForm, foundTable, foundList, foundProduct bool

	var traverse func(node *SemanticNode)
	traverse = func(node *SemanticNode) {
		switch node.Type {
		case FormType:
			foundForm = true
			if node.Properties["action"] != "/login" {
				t.Errorf("Expected action '/login', got '%v'", node.Properties["action"])
			}
			if node.Properties["method"] != "POST" {
				t.Errorf("Expected method 'POST', got '%v'", node.Properties["method"])
			}
			inputs, ok := node.Properties["inputs"].([]map[string]string)
			if !ok || len(inputs) != 2 {
				t.Errorf("Expected 2 inputs, got: %v", node.Properties["inputs"])
			} else {
				if inputs[0]["name"] != "username" || inputs[0]["type"] != "text" {
					t.Errorf("Unexpected first input: %v", inputs[0])
				}
				if inputs[1]["name"] != "password" || inputs[1]["type"] != "password" {
					t.Errorf("Unexpected second input: %v", inputs[1])
				}
			}
		case TableType:
			foundTable = true
			if node.Properties["rowsCount"].(int) != 3 {
				t.Errorf("Expected 3 rows, got %v", node.Properties["rowsCount"])
			}
			headers, ok := node.Properties["headers"].([]string)
			if !ok || len(headers) != 2 || headers[0] != "Name" || headers[1] != "Age" {
				t.Errorf("Expected headers [Name, Age], got: %v", node.Properties["headers"])
			}
		case ListType:
			foundList = true
		case ProductCardType:
			foundProduct = true
			if node.Properties["title"] != "Super Running Shoes" {
				t.Errorf("Expected title 'Super Running Shoes', got '%v'", node.Properties["title"])
			}
			if node.Properties["price"] != "$120.00" {
				t.Errorf("Expected price '$120.00', got '%v'", node.Properties["price"])
			}
			if node.Properties["imageUrl"] != "/images/shoes.png" {
				t.Errorf("Expected imageUrl '/images/shoes.png', got '%v'", node.Properties["imageUrl"])
			}
			if node.Properties["buttonText"] != "Add to Cart" {
				t.Errorf("Expected buttonText 'Add to Cart', got '%v'", node.Properties["buttonText"])
			}
			if node.Properties["url"] != "/products/shoes" {
				t.Errorf("Expected url '/products/shoes', got '%v'", node.Properties["url"])
			}
		}

		for _, child := range node.Children {
			traverse(child)
		}
	}

	traverse(graph)

	if !foundForm {
		t.Error("Expected to find Form semantic node, but didn't")
	}
	if !foundTable {
		t.Error("Expected to find Table semantic node, but didn't")
	}
	if !foundList {
		t.Error("Expected to find List semantic node, but didn't")
	}
	if !foundProduct {
		t.Error("Expected to find ProductCard semantic node, but didn't")
	}
}
