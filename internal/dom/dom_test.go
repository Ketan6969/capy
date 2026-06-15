package dom

import (
	"testing"

	"github.com/dop251/goja"
)

func TestDOMManipulation(t *testing.T) {
	doc := NewNode(DocumentNode, "#document")
	body := NewNode(ElementNode, "BODY")
	doc.AppendChild(body)

	div := NewNode(ElementNode, "DIV")
	div.SetAttribute("id", "main")
	div.SetAttribute("class", "container active")
	div.SetAttribute("data-custom", "hello")
	body.AppendChild(div)

	span := NewNode(ElementNode, "SPAN")
	span.SetTextContent("Hello World")
	div.AppendChild(span)

	if div.GetAttribute("id") != "main" {
		t.Errorf("Expected id to be 'main', got: %s", div.GetAttribute("id"))
	}
	if div.ClassName != "container active" {
		t.Errorf("Expected class to be 'container active', got: %s", div.ClassName)
	}

	if span.GetTextContent() != "Hello World" {
		t.Errorf("Expected textContent to be 'Hello World', got: %s", span.GetTextContent())
	}

	htmlStr := doc.GetOuterHTML()
	expected := `<body><div id="main" class="container active" data-custom="hello"><span>Hello World</span></div></body>`
	if htmlStr != expected {
		t.Errorf("Expected HTML: %s\nGot: %s", expected, htmlStr)
	}
}

func TestHTMLParsing(t *testing.T) {
	htmlContent := `<html><head><title>Test Page</title></head><body><div class="card" id="card-1"><p>Paragraph 1</p><p>Paragraph 2</p></div></body></html>`
	doc, err := ParseHTML(htmlContent)
	if err != nil {
		t.Fatalf("ParseHTML failed: %v", err)
	}

	titleNode := doc.QuerySelector("title")
	if titleNode == nil {
		t.Fatal("Title node not found")
	}
	if titleNode.GetTextContent() != "Test Page" {
		t.Errorf("Expected title text to be 'Test Page', got: %s", titleNode.GetTextContent())
	}

	paragraphs := doc.QuerySelectorAll("p")
	if len(paragraphs) != 2 {
		t.Errorf("Expected 2 paragraphs, got %d", len(paragraphs))
	}
	if paragraphs[1].GetTextContent() != "Paragraph 2" {
		t.Errorf("Expected second paragraph text to be 'Paragraph 2', got: %s", paragraphs[1].GetTextContent())
	}

	card := doc.QuerySelector("#card-1")
	if card == nil {
		t.Fatal("Card element not found by ID")
	}
	if card.ClassName != "card" {
		t.Errorf("Expected class name to be 'card', got: %s", card.ClassName)
	}
}

func TestJSIntegration(t *testing.T) {
	vm := goja.New()
	
	htmlContent := `<html><body><div id="app">Initial</div></body></html>`
	doc, err := ParseHTML(htmlContent)
	if err != nil {
		t.Fatalf("ParseHTML failed: %v", err)
	}

	SetupDOM(vm, doc, "")

	// Inject a basic event loop context run style check
	script := `
		const app = document.querySelector("#app");
		app.setAttribute("class", "loaded");
		app.innerHTML = "<span>Updated Content</span>";
		const navigatorUA = navigator.userAgent;
	`
	_, err = vm.RunString(script)
	if err != nil {
		t.Fatalf("JS run failed: %v", err)
	}

	appNode := doc.QuerySelector("#app")
	if appNode.ClassName != "loaded" {
		t.Errorf("Expected class to be updated to 'loaded', got: %s", appNode.ClassName)
	}

	childSpan := appNode.QuerySelector("span")
	if childSpan == nil {
		t.Fatal("Expected span child to be created via innerHTML")
	}
	if childSpan.GetTextContent() != "Updated Content" {
		t.Errorf("Expected textContent inside span to be 'Updated Content', got: %s", childSpan.GetTextContent())
	}
}

func TestJSWebAPIs(t *testing.T) {
	vm := goja.New()
	SetupDOM(vm, nil, "https://example.com:8080/home?user=alice#welcome")

	script := `
		// Test Location API
		const locHref = location.href;
		const locProto = location.protocol;
		const locPath = location.pathname;
		const docLocHref = document.location.href;

		// Test Storage API
		localStorage.setItem("testKey", "testValue");
		localStorage.setItem("numberKey", "123");
		const storeLength = localStorage.length;
		const retrievedVal = localStorage.getItem("testKey");
		localStorage.removeItem("numberKey");
		const newLength = localStorage.length;
	`
	_, err := vm.RunString(script)
	if err != nil {
		t.Fatalf("JS run failed: %v", err)
	}

	// Read variables back
	if vm.Get("locHref").String() != "https://example.com:8080/home?user=alice#welcome" {
		t.Errorf("Unexpected locHref: %s", vm.Get("locHref"))
	}
	if vm.Get("locProto").String() != "https:" {
		t.Errorf("Unexpected locProto: %s", vm.Get("locProto"))
	}
	if vm.Get("locPath").String() != "/home" {
		t.Errorf("Unexpected locPath: %s", vm.Get("locPath"))
	}
	if vm.Get("docLocHref").String() != "https://example.com:8080/home?user=alice#welcome" {
		t.Errorf("Unexpected docLocHref: %s", vm.Get("docLocHref"))
	}

	if vm.Get("storeLength").ToInteger() != 2 {
		t.Errorf("Expected length 2, got %d", vm.Get("storeLength").ToInteger())
	}
	if vm.Get("retrievedVal").String() != "testValue" {
		t.Errorf("Expected 'testValue', got '%s'", vm.Get("retrievedVal"))
	}
	if vm.Get("newLength").ToInteger() != 1 {
		t.Errorf("Expected length 1 after removal, got %d", vm.Get("newLength").ToInteger())
	}
}
