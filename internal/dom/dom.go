package dom

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/Ketan6969/capy/internal/polyfills"

	"github.com/Ketan6969/capy/internal/storage"
	"github.com/dop251/goja"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type NodeType = int

const (
	ElementNode      NodeType = 1
	TextNode         NodeType = 3
	DocumentNode     NodeType = 9
	DocumentFragment NodeType = 11
)

// Event represents a basic DOM Event.
type Event struct {
	Type string
}

var globalNodeUidCounter = 0

// Node represents a DOM Node in our unified DOM model.
type Node struct {
	Uid        int                    `json:"uid"`
	NodeType   NodeType               `json:"nodeType"`
	NodeName   string                 `json:"nodeName"`
	NodeValue  string                 `json:"nodeValue"`
	ParentNode *Node                  `json:"-"`
	ChildNodes []*Node                `json:"childNodes"`
	Attributes map[string]string      `json:"attributes,omitempty"`
	Id         string                 `json:"id,omitempty"`
	ClassName  string                 `json:"className,omitempty"`
	Location   *Location              `json:"location,omitempty"`
	Style      map[string]interface{} `json:"style,omitempty"`
	Type       string                 `json:"type,omitempty"`
	Src        string                 `json:"src,omitempty"`
	Href       string                 `json:"href,omitempty"`
	Value      string                 `json:"value,omitempty"`
	Name       string                 `json:"name,omitempty"`
	Width      interface{}            `json:"width,omitempty"`
	Height     interface{}            `json:"height,omitempty"`
	Checked    bool                   `json:"checked,omitempty"`
	Expandos   map[string]interface{} `json:"expandos,omitempty"`
}

// NewNode creates a new Node.
func NewNode(nodeType NodeType, nodeName string) *Node {
	globalNodeUidCounter++
	return &Node{
		Uid:        globalNodeUidCounter,
		NodeType:   nodeType,
		NodeName:   nodeName,
		Attributes: make(map[string]string),
		Expandos:   make(map[string]interface{}),
		Style:      make(map[string]interface{}),
	}
}

func (n *Node) AppendChild(child *Node) *Node {
	if child == nil {
		return nil
	}
	if child.ParentNode != nil {
		child.ParentNode.RemoveChild(child)
	}
	child.ParentNode = n
	n.ChildNodes = append(n.ChildNodes, child)
	return child
}

func (n *Node) InsertBefore(newNode, referenceNode *Node) *Node {
	if newNode == nil {
		return nil
	}
	if referenceNode == nil {
		return n.AppendChild(newNode)
	}

	if newNode.ParentNode != nil {
		newNode.ParentNode.RemoveChild(newNode)
	}

	for i, c := range n.ChildNodes {
		if c == referenceNode {
			n.ChildNodes = append(n.ChildNodes[:i], append([]*Node{newNode}, n.ChildNodes[i:]...)...)
			newNode.ParentNode = n
			return newNode
		}
	}
	return n.AppendChild(newNode)
}

func (n *Node) Remove() {
	if n.ParentNode != nil {
		n.ParentNode.RemoveChild(n)
	}
}

func (n *Node) RemoveChild(child *Node) *Node {
	for i, c := range n.ChildNodes {
		if c == child {
			n.ChildNodes = append(n.ChildNodes[:i], n.ChildNodes[i+1:]...)
			child.ParentNode = nil
			return child
		}
	}
	return nil
}

func (n *Node) Contains(other *Node) bool {
	if other == nil {
		return false
	}
	if n == other {
		return true
	}
	for _, c := range n.ChildNodes {
		if c.Contains(other) {
			return true
		}
	}
	return false
}

func (n *Node) GetTextContent() string {
	if n.NodeType == TextNode {
		return n.NodeValue
	}
	var sb strings.Builder
	for _, child := range n.ChildNodes {
		sb.WriteString(child.GetTextContent())
	}
	return sb.String()
}

func isBlockElement(nodeName string) bool {
	nn := strings.ToLower(nodeName)
	switch nn {
	case "div", "p", "h1", "h2", "h3", "h4", "h5", "h6", "li", "ul", "ol", "section", "article", "header", "footer", "blockquote", "table", "tr", "br", "nav", "main":
		return true
	}
	return false
}

func (n *Node) getInnerTextRecursive(sb *strings.Builder) {
	if n.NodeType == TextNode {
		sb.WriteString(n.NodeValue)
		return
	}
	if n.NodeType != ElementNode {
		return
	}

	nn := strings.ToLower(n.NodeName)
	if nn == "script" || nn == "style" || nn == "noscript" || nn == "head" {
		return
	}

	if n.Style != nil {
		if disp, ok := n.Style["display"]; ok {
			if ds, ok := disp.(string); ok && strings.TrimSpace(strings.ToLower(ds)) == "none" {
				return
			}
		}
		if vis, ok := n.Style["visibility"]; ok {
			if vs, ok := vis.(string); ok && strings.TrimSpace(strings.ToLower(vs)) == "hidden" {
				return
			}
		}
	}

	isBlock := isBlockElement(nn)
	if isBlock && nn != "br" {
		sb.WriteString("\n")
	}

	if nn == "br" {
		sb.WriteString("\n")
	} else {
		for _, child := range n.ChildNodes {
			child.getInnerTextRecursive(sb)
		}
	}

	if isBlock && nn != "br" {
		sb.WriteString("\n")
	}
}

func (n *Node) GetInnerText() string {
	var sb strings.Builder
	n.getInnerTextRecursive(&sb)

	text := sb.String()
	var lines []string
	for _, l := range strings.Split(text, "\n") {
		collapsed := strings.Join(strings.Fields(l), " ")
		if collapsed != "" {
			lines = append(lines, collapsed)
		}
	}
	return strings.Join(lines, "\n")
}

func (n *Node) SetTextContent(val string) {
	n.ChildNodes = nil
	if val != "" {
		textNode := NewNode(TextNode, "#text")
		textNode.NodeValue = val
		n.AppendChild(textNode)
	}
}

func (n *Node) SetAttribute(name, value string) {
	if n.Attributes == nil {
		n.Attributes = make(map[string]string)
	}
	name = strings.ToLower(name)
	if name == "id" {
		n.Id = value
	} else if name == "class" {
		n.ClassName = value
	} else if name == "style" {
		n.Attributes[name] = value
		if n.Style == nil {
			n.Style = make(map[string]interface{})
		}
		// Parse inline styles: "display: none; color: red;"
		rules := strings.Split(value, ";")
		for _, rule := range rules {
			parts := strings.SplitN(rule, ":", 2)
			if len(parts) == 2 {
				k := strings.TrimSpace(parts[0])
				v := strings.TrimSpace(parts[1])
				// Convert kebab-case to camelCase
				words := strings.Split(k, "-")
				camelKey := words[0]
				for i := 1; i < len(words); i++ {
					if len(words[i]) > 0 {
						camelKey += strings.ToUpper(words[i][:1]) + strings.ToLower(words[i][1:])
					}
				}
				n.Style[camelKey] = v
			}
		}
	} else {
		n.Attributes[name] = value
	}
}

func (n *Node) getDocumentURL() string {
	curr := n
	for curr != nil {
		if curr.NodeType == DocumentNode && curr.Location != nil {
			return curr.Location.Href
		}
		curr = curr.ParentNode
	}
	return ""
}

func (n *Node) GetBoundingClientRect() map[string]float64 {
	// A naive bounding client rect polyfill.
	// We check if the element or any of its ancestors are display: none.
	curr := n
	for curr != nil {
		if curr.Style != nil {
			if disp, ok := curr.Style["display"]; ok {
				if ds, ok := disp.(string); ok && strings.TrimSpace(strings.ToLower(ds)) == "none" {
					return map[string]float64{"x": 0, "y": 0, "width": 0, "height": 0, "top": 0, "right": 0, "bottom": 0, "left": 0}
				}
			}
		}
		curr = curr.ParentNode
	}

	// Basic sizing based on element type
	width := 0.0
	height := 0.0
	if isBlockElement(n.NodeName) {
		width = 1920.0 // Full window width mock
		height = 20.0  // Default mock height
	} else if n.NodeType == ElementNode {
		width = 100.0
		height = 20.0
	}

	return map[string]float64{
		"x":      0,
		"y":      0,
		"width":  width,
		"height": height,
		"top":    0,
		"right":  width,
		"bottom": height,
		"left":   0,
	}
}

func (n *Node) GetAttribute(name string) string {
	if n.Attributes == nil {
		return ""
	}
	name = strings.ToLower(name)
	if name == "id" {
		return n.Id
	} else if name == "class" {
		return n.ClassName
	}

	val, ok := n.Attributes[name]
	if !ok {
		return ""
	}

	if (name == "href" || name == "src") && val != "" {
		docURL := n.getDocumentURL()
		if docURL != "" {
			base, err := url.Parse(docURL)
			if err == nil {
				ref, err := url.Parse(val)
				if err == nil {
					return base.ResolveReference(ref).String()
				}
			}
		}
	}
	return val
}

func (n *Node) HasAttribute(name string) bool {
	if n.Attributes == nil {
		return false
	}
	name = strings.ToLower(name)
	if name == "id" {
		return n.Id != ""
	} else if name == "class" {
		return n.ClassName != ""
	}
	_, ok := n.Attributes[name]
	return ok
}

func (n *Node) RemoveAttribute(name string) {
	if n.Attributes == nil {
		return
	}
	name = strings.ToLower(name)
	if name == "id" {
		n.Id = ""
	} else if name == "class" {
		n.ClassName = ""
	} else {
		delete(n.Attributes, name)
	}
}

func (n *Node) GetElementById(id string) *Node {
	var dfs func(node *Node) *Node
	dfs = func(node *Node) *Node {
		if node.NodeType == ElementNode && node.Id == id {
			return node
		}
		for _, child := range node.ChildNodes {
			if res := dfs(child); res != nil {
				return res
			}
		}
		return nil
	}
	return dfs(n)
}

func (n *Node) GetElementsByClassName(className string) []*Node {
	var results []*Node
	var dfs func(node *Node)
	dfs = func(node *Node) {
		if node.NodeType == ElementNode {
			classes := strings.Fields(node.ClassName)
			for _, c := range classes {
				if c == className {
					results = append(results, node)
					break
				}
			}
		}
		for _, child := range node.ChildNodes {
			dfs(child)
		}
	}
	dfs(n)
	return results
}

func (n *Node) GetElementsByTagName(tagName string) []*Node {
	var results []*Node
	upperTag := strings.ToUpper(tagName)
	var dfs func(node *Node)
	dfs = func(node *Node) {
		if node.NodeType == ElementNode && (upperTag == "*" || node.NodeName == upperTag) {
			results = append(results, node)
		}
		for _, child := range node.ChildNodes {
			dfs(child)
		}
	}
	dfs(n)
	return results
}

func (n *Node) QuerySelector(selector string) *Node {
	results := n.QuerySelectorAll(selector)
	if len(results) > 0 {
		return results[0]
	}
	return nil
}

func (n *Node) QuerySelectorAll(selector string) []*Node {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil
	}

	// Handle multiple selectors separated by comma
	if strings.Contains(selector, ",") {
		var results []*Node
		seen := make(map[int]bool)
		for _, part := range strings.Split(selector, ",") {
			for _, res := range n.QuerySelectorAll(part) {
				if !seen[res.Uid] {
					seen[res.Uid] = true
					results = append(results, res)
				}
			}
		}
		return results
	}

	// Basic descendant combinator split by space
	parts := strings.Fields(selector)
	if len(parts) == 0 {
		return nil
	}

	matchPart := func(part string) func(*Node) bool {
		return func(node *Node) bool {
			if node.NodeType != ElementNode {
				return false
			}
			p := part

			// 1. Tag name
			tagName := ""
			idx := strings.IndexAny(p, "#.[")
			if idx == -1 {
				tagName = p
				p = ""
			} else if idx > 0 {
				tagName = p[:idx]
				p = p[idx:]
			}
			if tagName != "" && tagName != "*" {
				if node.NodeName != strings.ToUpper(tagName) {
					return false
				}
			}

			// 2. ID, Classes, Attributes
			for len(p) > 0 {
				if p[0] == '#' {
					p = p[1:]
					idx = strings.IndexAny(p, "#.[")
					id := p
					if idx != -1 {
						id = p[:idx]
						p = p[idx:]
					} else {
						p = ""
					}
					if node.Id != id {
						return false
					}
				} else if p[0] == '.' {
					p = p[1:]
					idx = strings.IndexAny(p, "#.[")
					cls := p
					if idx != -1 {
						cls = p[:idx]
						p = p[idx:]
					} else {
						p = ""
					}
					found := false
					for _, c := range strings.Fields(node.ClassName) {
						if c == cls {
							found = true
							break
						}
					}
					if !found {
						return false
					}
				} else if p[0] == '[' {
					idx = strings.Index(p, "]")
					if idx == -1 {
						break
					}
					attrExpr := p[1:idx]
					p = p[idx+1:]

					eqIdx := strings.Index(attrExpr, "=")
					if eqIdx == -1 {
						if !node.HasAttribute(attrExpr) {
							return false
						}
					} else {
						attrName := attrExpr[:eqIdx]
						attrVal := strings.Trim(attrExpr[eqIdx+1:], "\"'")
						if node.GetAttribute(attrName) != attrVal {
							return false
						}
					}
				} else {
					break // unsupported syntax
				}
			}
			return true
		}
	}

	matchers := make([]func(*Node) bool, len(parts))
	for i, part := range parts {
		matchers[i] = matchPart(part)
	}

	var results []*Node
	var dfs func(node *Node, matchIndex int)
	dfs = func(node *Node, matchIndex int) {
		if matchIndex == len(matchers) {
			results = append(results, node)
			return
		}
		matcher := matchers[matchIndex]
		for _, child := range node.ChildNodes {
			if matcher(child) {
				if matchIndex == len(matchers)-1 {
					results = append(results, child)
				} else {
					dfs(child, matchIndex+1)
				}
			}
			dfs(child, matchIndex)
		}
	}

	dfs(n, 0)

	// Deduplicate in case multiple paths matched
	dedup := make([]*Node, 0, len(results))
	seen := make(map[int]bool)
	for _, r := range results {
		if !seen[r.Uid] {
			seen[r.Uid] = true
			dedup = append(dedup, r)
		}
	}

	return dedup
}

func (n *Node) GetOuterHTML() string {
	if n.NodeType == TextNode {
		return html.EscapeString(n.NodeValue)
	}
	if n.NodeType == DocumentNode {
		var sb strings.Builder
		for _, child := range n.ChildNodes {
			sb.WriteString(child.GetOuterHTML())
		}
		return sb.String()
	}

	var sb strings.Builder
	tagName := strings.ToLower(n.NodeName)
	sb.WriteString("<")
	sb.WriteString(tagName)
	if n.Id != "" {
		sb.WriteString(fmt.Sprintf(` id="%s"`, html.EscapeString(n.Id)))
	}
	if n.ClassName != "" {
		sb.WriteString(fmt.Sprintf(` class="%s"`, html.EscapeString(n.ClassName)))
	}
	if n.Attributes != nil {
		for k, v := range n.Attributes {
			sb.WriteString(fmt.Sprintf(` %s="%s"`, k, html.EscapeString(v)))
		}
	}
	sb.WriteString(">")

	selfClosing := map[string]bool{
		"img": true, "br": true, "hr": true, "input": true, "meta": true, "link": true,
	}
	if selfClosing[tagName] {
		return sb.String()
	}

	for _, child := range n.ChildNodes {
		sb.WriteString(child.GetOuterHTML())
	}
	sb.WriteString(fmt.Sprintf("</%s>", tagName))
	return sb.String()
}

func (n *Node) GetInnerHTML() string {
	var sb strings.Builder
	for _, child := range n.ChildNodes {
		sb.WriteString(child.GetOuterHTML())
	}
	return sb.String()
}

func (n *Node) SetInnerHTML(htmlStr string) {
	n.ChildNodes = nil
	tagLower := strings.ToLower(n.NodeName)
	contextNode := &html.Node{
		Type:     html.ElementNode,
		Data:     tagLower,
		DataAtom: atom.Lookup([]byte(tagLower)),
	}
	nodes, err := html.ParseFragment(strings.NewReader(htmlStr), contextNode)
	if err != nil {
		return
	}

	for _, childNetNode := range nodes {
		childNode := convertNetNode(childNetNode)
		n.AppendChild(childNode)
	}
}

// CreateElement creates a new Element Node.
func (n *Node) CreateElement(tagName string) *Node {
	return NewNode(ElementNode, strings.ToUpper(tagName))
}

// CreateDocumentFragment creates a new DocumentFragment Node.
func (n *Node) CreateDocumentFragment() *Node {
	return NewNode(DocumentFragment, "#document-fragment")
}

// CloneNode returns a copy of the node.
func (n *Node) CloneNode(deep bool) *Node {
	clone := NewNode(n.NodeType, n.NodeName)
	clone.NodeValue = n.NodeValue
	clone.Id = n.Id
	clone.ClassName = n.ClassName
	for k, v := range n.Attributes {
		clone.SetAttribute(k, v)
	}
	if deep {
		for _, child := range n.ChildNodes {
			clone.AppendChild(child.CloneNode(true))
		}
	}
	return clone
}

// CreateTextNode creates a new Text Node.
func (n *Node) CreateTextNode(text string) *Node {
	node := NewNode(TextNode, "#text")
	node.NodeValue = text
	return node
}

// GetBody returns the body element if called on a Document node.
func (n *Node) GetBody() *Node {
	if n.NodeType != DocumentNode {
		return nil
	}
	return n.QuerySelector("body")
}

// GetHead returns the head element if called on a Document node.
func (n *Node) GetHead() *Node {
	if n.NodeType != DocumentNode {
		return nil
	}
	return n.QuerySelector("head")
}

// GetDocumentElement returns the html element if called on a Document node.
func (n *Node) GetDocumentElement() *Node {
	if n.NodeType != DocumentNode {
		return nil
	}
	return n.QuerySelector("html")
}

// ParseHTML parses an HTML string and returns a root Node (typically document or fragment).
func ParseHTML(htmlStr string) (*Node, error) {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return nil, err
	}
	return convertNetNode(doc), nil
}

func convertNetNode(n *html.Node) *Node {
	var nodeType NodeType
	var nodeName string
	var nodeValue string

	switch n.Type {
	case html.DocumentNode:
		nodeType = DocumentNode
		nodeName = "#document"
	case html.ElementNode:
		nodeType = ElementNode
		nodeName = strings.ToUpper(n.Data)
	case html.TextNode:
		nodeType = TextNode
		nodeName = "#text"
		nodeValue = n.Data
	default:
		nodeType = TextNode
		nodeName = "#comment"
		nodeValue = n.Data
	}

	globalNodeUidCounter++
	converted := &Node{
		Uid:        globalNodeUidCounter,
		NodeType:   nodeType,
		NodeName:   nodeName,
		NodeValue:  nodeValue,
		Attributes: make(map[string]string),
		Expandos:   make(map[string]interface{}),
	}

	if nodeType == ElementNode {
		for _, attr := range n.Attr {
			converted.SetAttribute(attr.Key, attr.Val)
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		childConverted := convertNetNode(c)
		converted.AppendChild(childConverted)
	}

	return converted
}

// SetupDOM initializes a standard DOM structure inside a Goja runtime environment.

// DispatchLifecycleEvents triggers the standard DOMContentLoaded and load events.
func DispatchLifecycleEvents(vm *goja.Runtime) {
	vm.RunString(`
		(function() {
			document.readyState = "interactive";
			const dcl = new Event("DOMContentLoaded");
			document.dispatchEvent(dcl);
			
			document.readyState = "complete";
			const load = new Event("load");
			window.dispatchEvent(load);
		})();
	`)
}

// SetupCookies links the Goja document object to the network cookie jar.
func SetupCookies(vm *goja.Runtime, jar http.CookieJar, docURL string) {
	if jar == nil || docURL == "" {
		return
	}
	u, err := url.Parse(docURL)
	if err != nil {
		return
	}
	if u.Host == "" {
		u, _ = url.Parse("http://localhost/")
	}

	vm.Set("_goGetCookies", func() string {
		cookies := jar.Cookies(u)
		var sb strings.Builder
		for i, c := range cookies {
			if i > 0 {
				sb.WriteString("; ")
			}
			sb.WriteString(c.Name + "=" + c.Value)
		}
		return sb.String()
	})

	vm.Set("_goSetCookie", func(cookieStr string) {
		// A simple parser for the first key=value
		parts := strings.SplitN(cookieStr, ";", 2)
		kv := strings.SplitN(strings.TrimSpace(parts[0]), "=", 2)
		if len(kv) == 2 {
			c := &http.Cookie{
				Name:  strings.TrimSpace(kv[0]),
				Value: strings.TrimSpace(kv[1]),
			}
			jar.SetCookies(u, []*http.Cookie{c})
		}
	})

	vm.RunString(polyfills.CookiesScript)
}

func SetupDOM(vm *goja.Runtime, documentRoot *Node, docURL string) {
	// Map Go method names to camelCase in JavaScript
	vm.SetFieldNameMapper(goja.UncapFieldNameMapper())

	if documentRoot == nil {
		documentRoot = &Node{
			NodeType: DocumentNode,
			NodeName: "#document",
		}
		htmlNode := NewNode(ElementNode, "HTML")
		headNode := NewNode(ElementNode, "HEAD")
		bodyNode := NewNode(ElementNode, "BODY")
		htmlNode.AppendChild(headNode)
		htmlNode.AppendChild(bodyNode)
		documentRoot.AppendChild(htmlNode)
	}

	navigator := vm.NewObject()
	navigator.Set("userAgent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	navigator.Set("platform", "Linux")
	navigator.Set("language", "en-US")
	vm.Set("navigator", navigator)

	window := vm.GlobalObject()
	window.Set("window", window)
	window.Set("self", window)
	window.Set("document", documentRoot)

	// Expose localStorage, sessionStorage, and location
	localStorage := storage.NewStorage()
	sessionStorage := storage.NewStorage()
	location := NewLocation(docURL)

	window.Set("localStorage", localStorage)
	window.Set("sessionStorage", sessionStorage)
	window.Set("location", location)
	if documentRoot != nil {
		documentRoot.Location = location

		// Pre-resolve all URLs so goja's mapped Href and Src fields contain absolute paths
		var resolveURLs func(n *Node)
		resolveURLs = func(n *Node) {
			if href := n.GetAttribute("href"); href != "" {
				n.Href = href
			}
			if src := n.GetAttribute("src"); src != "" {
				n.Src = src
			}
			for _, child := range n.ChildNodes {
				resolveURLs(child)
			}
		}
		resolveURLs(documentRoot)
		window.Set("document", documentRoot)
	}

	// Define standard DOM getters/setters on the *Node prototype
	_, err := vm.RunString(polyfills.DomScript)
	if err != nil {
		slog.Error("Error executing dom polyfills", "error", err)
	}
	_, err = vm.RunString(polyfills.XhrScript)
	if err != nil {
		slog.Error("Error executing xhr polyfills", "error", err)
	}
}
