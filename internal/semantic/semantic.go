package semantic

import (
	"regexp"
	"strings"

	"github.com/Ketan6969/capy/internal/dom"
)

// ComponentType represents the class of a semantic component.
type ComponentType string

const (
	ProductCardType ComponentType = "ProductCard"
	ListType        ComponentType = "List"
	TableType       ComponentType = "Table"
	FormType        ComponentType = "Form"
	ArticleType     ComponentType = "Article"
	TextSectionType ComponentType = "TextSection"
	UnknownType     ComponentType = "Unknown"
)

// SemanticNode represents a component in our machine-readable page graph.
type SemanticNode struct {
	Type       ComponentType          `json:"type"`
	Id         string                 `json:"id,omitempty"`
	ClassName  string                 `json:"className,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Children   []*SemanticNode        `json:"children,omitempty"`
}

var priceRegex = regexp.MustCompile(`(?i)([\$\xA2-\xA5\x{058F}\x{060B}\x{09F2}\x{09F3}\x{09FB}\x{0AF1}\x{0BF9}\x{0E3F}\x{17DB}\x{20A0}-\x{20BD}\x{20C0}\x{20C1}\x{A838}\x{FDFC}\x{FE69}\x{FF04}\x{FFE0}\x{FFE1}\x{FFE5}\x{FFE6}]|\b(usd|eur|gbp|inr|cad|aud|cny|jpy)\b)\s?\d+([\.,]\d{2})?`)
var ctaRegex = regexp.MustCompile(`(?i)\b(buy|add to cart|cart|checkout|purchase|order|get|shop|add)\b`)

// ParseSemanticGraph parses a raw DOM tree and returns the root of the Semantic Graph.
func ParseSemanticGraph(root *dom.Node) *SemanticNode {
	if root == nil {
		return nil
	}
	return classifyNode(root)
}

func classifyNode(node *dom.Node) *SemanticNode {
	if node.NodeType == dom.TextNode {
		return nil
	}

	// 1. Check if Table
	if node.NodeName == "TABLE" {
		rows := node.QuerySelectorAll("tr")
		headers := node.QuerySelectorAll("th")
		headerTexts := []string{}
		for _, h := range headers {
			headerTexts = append(headerTexts, strings.TrimSpace(h.GetTextContent()))
		}
		return &SemanticNode{
			Type:      TableType,
			Id:        node.Id,
			ClassName: node.ClassName,
			Properties: map[string]interface{}{
				"rowsCount": len(rows),
				"headers":   headerTexts,
			},
			Children: classifyChildren(node.ChildNodes),
		}
	}

	// 2. Check if Form
	if node.NodeName == "FORM" {
		inputs := node.QuerySelectorAll("input")
		inputFields := []map[string]string{}
		for _, in := range inputs {
			name := in.GetAttribute("name")
			inType := in.GetAttribute("type")
			if inType == "" {
				inType = "text"
			}
			inputFields = append(inputFields, map[string]string{
				"name": name,
				"type": inType,
			})
		}
		return &SemanticNode{
			Type:      FormType,
			Id:        node.Id,
			ClassName: node.ClassName,
			Properties: map[string]interface{}{
				"action": node.GetAttribute("action"),
				"method": node.GetAttribute("method"),
				"inputs": inputFields,
			},
			Children: classifyChildren(node.ChildNodes),
		}
	}

	// 3. Check if Product Card (with bottom-up validation)
	if isProductCard(node) {
		hasProductDescendant := false
		for _, child := range node.ChildNodes {
			if hasProductCardDescendant(child) {
				hasProductDescendant = true
				break
			}
		}
		if !hasProductDescendant {
			title, price, img, cta, href := extractProductProperties(node)
			return &SemanticNode{
				Type:      ProductCardType,
				Id:        node.Id,
				ClassName: node.ClassName,
				Properties: map[string]interface{}{
					"title":      title,
					"price":      price,
					"imageUrl":   img,
					"buttonText": cta,
					"url":        href,
				},
			}
		}
	}

	// 4. Check if List
	if node.NodeName == "UL" || node.NodeName == "OL" {
		return &SemanticNode{
			Type:      ListType,
			Id:        node.Id,
			ClassName: node.ClassName,
			Children:  classifyChildren(node.ChildNodes),
		}
	}

	// 5. Check if Article
	if isArticle(node) {
		var titleNode *dom.Node
		for _, tag := range []string{"h1", "h2", "h3", "h4"} {
			if found := node.QuerySelector(tag); found != nil {
				titleNode = found
				break
			}
		}
		title := ""
		if titleNode != nil {
			title = strings.TrimSpace(titleNode.GetTextContent())
		}
		return &SemanticNode{
			Type:      ArticleType,
			Id:        node.Id,
			ClassName: node.ClassName,
			Properties: map[string]interface{}{
				"title": title,
				"text":  strings.TrimSpace(node.GetTextContent()),
			},
		}
	}

	// Generic container fallback
	children := classifyChildren(node.ChildNodes)
	if len(children) > 0 {
		return &SemanticNode{
			Type:      UnknownType,
			Id:        node.Id,
			ClassName: node.ClassName,
			Children:  children,
		}
	}

	return nil
}

func classifyChildren(childNodes []*dom.Node) []*SemanticNode {
	var list []*SemanticNode
	for _, child := range childNodes {
		sem := classifyNode(child)
		if sem != nil {
			list = append(list, sem)
		}
	}
	return list
}

func isProductCard(node *dom.Node) bool {
	images := node.QuerySelectorAll("img")
	if len(images) == 0 {
		return false
	}

	text := node.GetTextContent()
	return priceRegex.MatchString(text)
}

func hasProductCardDescendant(node *dom.Node) bool {
	if isProductCard(node) {
		return true
	}
	for _, child := range node.ChildNodes {
		if hasProductCardDescendant(child) {
			return true
		}
	}
	return false
}

func extractProductProperties(node *dom.Node) (title, price, img, cta, href string) {
	images := node.QuerySelectorAll("img")
	if len(images) > 0 {
		img = images[0].GetAttribute("src")
	}

	text := node.GetTextContent()
	priceMatch := priceRegex.FindString(text)
	price = strings.TrimSpace(priceMatch)

	buttons := node.QuerySelectorAll("button")
	buttons = append(buttons, node.QuerySelectorAll("a")...)
	for _, b := range buttons {
		bText := b.GetTextContent()
		if ctaRegex.MatchString(bText) {
			cta = strings.TrimSpace(bText)
			if b.NodeName == "A" {
				href = b.GetAttribute("href")
			}
			break
		}
	}

	var headings []*dom.Node
	for _, tag := range []string{"h1", "h2", "h3", "h4", "h5", "h6", "strong", "b"} {
		headings = append(headings, node.QuerySelectorAll(tag)...)
	}
	if len(headings) > 0 {
		title = strings.TrimSpace(headings[0].GetTextContent())
	} else {
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !priceRegex.MatchString(line) && len(line) > 3 {
				title = line
				break
			}
		}
	}

	return
}

func isArticle(node *dom.Node) bool {
	if node.NodeName == "ARTICLE" {
		return true
	}
	if node.NodeName == "P" {
		return len(strings.TrimSpace(node.GetTextContent())) > 60
	}
	return false
}
