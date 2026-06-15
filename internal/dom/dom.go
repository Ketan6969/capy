package dom

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/browserless/runtime/internal/storage"
	"github.com/dop251/goja"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type NodeType = int

const (
	ElementNode       NodeType = 1
	TextNode          NodeType = 3
	DocumentNode      NodeType = 9
	DocumentFragment NodeType = 11
)

// Event represents a basic DOM Event.
type Event struct {
	Type string
}

// EventListener is a callback function for DOM Events.
type EventListener func(event *Event)

// EventTarget handles registration and dispatching of events.
type EventTarget struct {
	listeners map[string][]EventListener
}

// NewEventTarget creates a new EventTarget.
func NewEventTarget() *EventTarget {
	return &EventTarget{
		listeners: make(map[string][]EventListener),
	}
}

func (et *EventTarget) AddEventListener(eventType string, listener EventListener) {
	if et.listeners == nil {
		et.listeners = make(map[string][]EventListener)
	}
	et.listeners[eventType] = append(et.listeners[eventType], listener)
}

func (et *EventTarget) DispatchEvent(event *Event) bool {
	if et.listeners == nil {
		return true
	}
	listeners := et.listeners[event.Type]
	for _, l := range listeners {
		l(event)
	}
	return true
}

var globalNodeUidCounter = 0

// Node represents a DOM Node in our unified DOM model.
type Node struct {
	*EventTarget
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
}

// NewNode creates a new Node.
func NewNode(nodeType NodeType, nodeName string) *Node {
	globalNodeUidCounter++
	return &Node{
		Uid:         globalNodeUidCounter,
		EventTarget: NewEventTarget(),
		NodeType:    nodeType,
		NodeName:    nodeName,
		Attributes:  make(map[string]string),
		Style:       make(map[string]interface{}),
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

	if styleStr, ok := n.Attributes["style"]; ok {
		s := strings.ToLower(styleStr)
		if strings.Contains(s, "display: none") || strings.Contains(s, "display:none") ||
			strings.Contains(s, "visibility: hidden") || strings.Contains(s, "visibility:hidden") {
			return
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
		Uid:         globalNodeUidCounter,
		EventTarget: NewEventTarget(),
		NodeType:    nodeType,
		NodeName:    nodeName,
		NodeValue:   nodeValue,
		Attributes:  make(map[string]string),
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
func SetupDOM(vm *goja.Runtime, documentRoot *Node, docURL string) {
	// Map Go method names to camelCase in JavaScript
	vm.SetFieldNameMapper(goja.UncapFieldNameMapper())

	if documentRoot == nil {
		documentRoot = &Node{
			EventTarget: NewEventTarget(),
			NodeType:    DocumentNode,
			NodeName:    "#document",
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
	setupScript := `
		(function() {
			if (typeof document === 'undefined') return;
			const proto = Object.getPrototypeOf(document);
			if (!proto) return;

			// Add generic polyfills for global APIs that libraries expect
			globalThis.window = globalThis;
			globalThis.setTimeout = function(cb) { if (cb) { try { cb(); } catch(e) {} } return 1; };
			globalThis.clearTimeout = function() {};
			globalThis.setInterval = function() { return 1; };
			globalThis.clearInterval = function() {};
			globalThis.requestAnimationFrame = function(cb) { return setTimeout(cb, 16); };
			globalThis.cancelAnimationFrame = function() {};

			// performance API stub
			if (typeof performance === 'undefined') {
				const _perfStart = Date.now();
				globalThis.performance = {
					now: function() { return Date.now() - _perfStart; },
					mark: function() {},
					measure: function() {},
					clearMarks: function() {},
					clearMeasures: function() {},
					getEntriesByName: function() { return []; },
					getEntriesByType: function() { return []; },
					getEntries: function() { return []; },
					timing: { navigationStart: Date.now() },
					navigation: { type: 0, redirectCount: 0 },
					resourceTimingBufferSize: 150,
					setResourceTimingBufferSize: function() {},
					clearResourceTimings: function() {},
					observe: function() {}
				};
			}

			// XMLHttpRequest stub
			if (typeof XMLHttpRequest === 'undefined') {
				globalThis.XMLHttpRequest = function() {
					this.readyState = 0;
					this.status = 0;
					this.responseText = '';
					this.onreadystatechange = null;
					this.onload = null;
					this.onerror = null;
				};
				globalThis.XMLHttpRequest.prototype.open = function() {};
				globalThis.XMLHttpRequest.prototype.send = function() {};
				globalThis.XMLHttpRequest.prototype.setRequestHeader = function() {};
				globalThis.XMLHttpRequest.prototype.abort = function() {};
				globalThis.XMLHttpRequest.prototype.addEventListener = function() {};
				globalThis.XMLHttpRequest.UNSENT = 0;
				globalThis.XMLHttpRequest.OPENED = 1;
				globalThis.XMLHttpRequest.HEADERS_RECEIVED = 2;
				globalThis.XMLHttpRequest.LOADING = 3;
				globalThis.XMLHttpRequest.DONE = 4;
			}

			// screen stub
			if (typeof screen === 'undefined') {
				globalThis.screen = { width: 1920, height: 1080, availWidth: 1920, availHeight: 1080, colorDepth: 24, pixelDepth: 24 };
			}
			if (typeof devicePixelRatio === 'undefined') { globalThis.devicePixelRatio = 1; }
			if (typeof matchMedia === 'undefined') { globalThis.matchMedia = function() { return { matches: false, addListener: function(){}, removeListener: function(){}, addEventListener: function(){} }; }; }

			// Wrap Object.defineProperty to prevent Goja Host Object crashes
			const origDefineProperty = Object.defineProperty;
			const defineFallback = new WeakMap();
			Object.defineProperty = function(obj, prop, descriptor) {
				try {
					return origDefineProperty(obj, prop, descriptor);
				} catch (e) {
					const msg = e.message || '';
					// Silently absorb errors caused by Goja's host-object limitations:
					//   "cannot be made configurable" — non-configurable Go-backed property
					//   "getter must be a function"   — accessor descriptor on a host object
					//   "setter must be a function"   — same, for setters
					if (msg.includes('cannot be made configurable')) {
						if (descriptor && 'value' in descriptor) {
							try { obj[prop] = descriptor.value; } catch(err) {}
							let data = defineFallback.get(obj);
							if (!data) { data = {}; defineFallback.set(obj, data); }
							data[prop] = descriptor.value;
						}
						return obj;
					}
					if (msg.includes('getter must be a function') || msg.includes('setter must be a function')) {
						// Accessor descriptor on a host object — skip silently
						return obj;
					}
					throw e;
				}
			};

			// jQuery interceptor to support expandos on Host Objects
			const jqDataStore = {};
			let originalJQuery = undefined;
			
			function defineExpando(expando) {
				if (proto.__jqPatched && proto.__jqPatched[expando]) return;
				if (!proto.__jqPatched) proto.__jqPatched = {};
				proto.__jqPatched[expando] = true;
				
				for (let i = 0; i <= 2000; i++) {
					const prop = i === 0 ? expando : expando + i;
					Object.defineProperty(proto, prop, {
						get: function() {
							const uid = this.uid;
							if (!uid) return undefined;
							return jqDataStore[uid] ? jqDataStore[uid][prop] : undefined;
						},
						set: function(v) {
							const uid = this.uid;
							if (!uid) {
								Object.defineProperty(this, prop, {value: v, writable: true, configurable: true});
								return;
							}
							if (!jqDataStore[uid]) jqDataStore[uid] = {};
							jqDataStore[uid][prop] = v;
						},
						configurable: true,
						enumerable: false
					});
				}
			}

			Object.defineProperty(globalThis, 'jQuery', {
				get: function() { return originalJQuery; },
				set: function(val) {
					originalJQuery = val;
					if (val && val.expando) {
						defineExpando(val.expando);
					}
				},
				configurable: true
			});
			
			let originalDollar = undefined;
			Object.defineProperty(globalThis, '$', {
				get: function() { return originalDollar !== undefined ? originalDollar : originalJQuery; },
				set: function(val) {
					originalDollar = val;
				},
				configurable: true
			});

			Object.defineProperty(proto, 'implementation', {
				get() {
					if (Number(this.nodeType) === 9) { // DocumentNode
						return {
							createHTMLDocument: function() {
								const doc = {
									nodeType: 9,
									nodeName: '#document',
									childNodes: [],
									createElement: function(tag) { return document.createElement(tag); },
									createDocumentFragment: function() { return document.createDocumentFragment(); },
									body: document.createElement('body')
								};
								return doc;
							}
						};
					}
					return undefined;
				},
				configurable: true,
				enumerable: false
			});

			globalThis.Event = class Event {
				constructor(type) {
					this.type = type;
				}
			};

			proto.addEventListener = function(type, callback) {
				if (!this._listeners) {
					this._listeners = {};
				}
				if (!this._listeners[type]) {
					this._listeners[type] = [];
				}
				this._listeners[type].push(callback);
			};

			proto.dispatchEvent = function(event) {
				if (!event || !event.type) return true;
				if (this._listeners && this._listeners[event.type]) {
					const list = this._listeners[event.type];
					for (let i = 0; i < list.length; i++) {
						try {
							list[i](event);
						} catch (e) {
							console.error("Event listener error:", e);
						}
					}
				}
				return true;
			};

			Object.defineProperty(proto, 'innerHTML', {
				get() { 
					if (typeof this.getInnerHTML === 'function') {
						return this.getInnerHTML(); 
					}
					return undefined;
				},
				set(val) { 
					if (typeof this.setInnerHTML === 'function') {
						this.setInnerHTML(val); 
					}
				},
				configurable: true,
				enumerable: false
			});

			Object.defineProperty(proto, 'outerHTML', {
				get() { 
					if (typeof this.getOuterHTML === 'function') {
						return this.getOuterHTML(); 
					}
					return undefined;
				},
				set(val) {},
				configurable: true,
				enumerable: false
			});
			Object.defineProperty(proto, 'innerText', {
				get() { 
					if (typeof this.getInnerText === 'function') {
						return this.getInnerText(); 
					}
					return undefined;
				},
				set(val) { 
					if (typeof this.setTextContent === 'function') {
						this.setTextContent(val); 
					}
				},
				configurable: true,
				enumerable: false
			});


			Object.defineProperty(proto, 'textContent', {
				get() { 
					if (typeof this.getTextContent === 'function') {
						return this.getTextContent(); 
					}
					return undefined;
				},
				set(val) { 
					if (typeof this.setTextContent === 'function') {
						this.setTextContent(val); 
					}
				},
				configurable: true,
				enumerable: false
			});

			Object.defineProperty(proto, 'firstChild', {
				get() { 
					if (this.childNodes && this.childNodes.length > 0) {
						return this.childNodes[0];
					}
					return null;
				},
				set(val) {},
				configurable: true,
				enumerable: false
			});

			Object.defineProperty(proto, 'lastChild', {
				get() { 
					if (this.childNodes && this.childNodes.length > 0) {
						return this.childNodes[this.childNodes.length - 1];
					}
					return null;
				},
				set(val) {},
				configurable: true,
				enumerable: false
			});

			proto.remove = function() {
				if (this.parentNode) {
					this.parentNode.removeChild(this);
				}
			};

			proto.getBoundingClientRect = function() {
				return { x: 0, y: 0, width: 0, height: 0, top: 0, right: 0, bottom: 0, left: 0 };
			};

			Object.defineProperty(proto, 'classList', {
				get() {
					const node = this;
					return {
						add: function(...classes) {
							const current = (node.className || '').split(' ').filter(c => c);
							for (const cls of classes) {
								if (current.indexOf(cls) === -1) current.push(cls);
							}
							node.className = current.join(' ');
						},
						remove: function(...classes) {
							const current = (node.className || '').split(' ').filter(c => c);
							const updated = current.filter(c => classes.indexOf(c) === -1);
							node.className = updated.join(' ');
						},
						toggle: function(cls) {
							const current = (node.className || '').split(' ').filter(c => c);
							const idx = current.indexOf(cls);
							if (idx === -1) {
								current.push(cls);
								node.className = current.join(' ');
								return true;
							} else {
								current.splice(idx, 1);
								node.className = current.join(' ');
								return false;
							}
						},
						contains: function(cls) {
							const current = (node.className || '').split(' ').filter(c => c);
							return current.indexOf(cls) !== -1;
						}
					};
				},
				configurable: true,
				enumerable: false
			});

			if (typeof Intl === 'undefined') {
				globalThis.Intl = {
					DateTimeFormat: function() {
						return { format: function() { return ''; }, resolvedOptions: function() { return { locale: 'en-US' }; } };
					},
					NumberFormat: function() {
						return { format: function(n) { return n ? n.toString() : ''; }, resolvedOptions: function() { return { locale: 'en-US' }; } };
					}
				};
			}

			globalThis.Window = function Window() {};

			globalThis.__raf_count__ = 0;
			globalThis.requestAnimationFrame = function(callback) {
				if (globalThis.__raf_count__++ < 10) {
					return setTimeout(callback, 10);
				}
				return 0;
			};
			globalThis.cancelAnimationFrame = function(id) {
				clearTimeout(id);
			};

			globalThis.getComputedStyle = function(el) {
				return {
					getPropertyValue: function() { return ''; },
					setProperty: function() {},
					removeProperty: function() {}
				};
			};

			globalThis.HTMLCanvasElement = function() {};
			globalThis.HTMLCanvasElement.prototype.getContext = function() {
				return {
					fillRect: function() {},
					clearRect: function() {},
					getImageData: function() { return { data: [] }; },
					putImageData: function() {},
					createImageData: function() { return { data: [] }; },
					setTransform: function() {},
					drawImage: function() {},
					save: function() {},
					fillText: function() {},
					restore: function() {},
					beginPath: function() {},
					moveTo: function() {},
					lineTo: function() {},
					closePath: function() {},
					stroke: function() {},
					translate: function() {},
					scale: function() {},
					rotate: function() {},
					arc: function() {},
					fill: function() {},
					measureText: function() { return { width: 0 }; },
					transform: function() {},
					rect: function() {},
					clip: function() {}
				};
			};

			globalThis.IntersectionObserver = function() {
				this.observe = function() {};
				this.unobserve = function() {};
				this.disconnect = function() {};
			};

			globalThis.ResizeObserver = function() {
				this.observe = function() {};
				this.unobserve = function() {};
				this.disconnect = function() {};
			};

			globalThis.MutationObserver = function() {
				this.observe = function() {};
				this.disconnect = function() {};
				this.takeRecords = function() { return []; };
			};
			
			globalThis.PerformanceObserver = function() {
				this.observe = function() {};
				this.disconnect = function() {};
			};

			Object.defineProperty(proto, 'href', {
				get() { 
					if (typeof this.getAttribute === 'function') {
						return this.getAttribute('href'); 
					}
					return this.Href || '';
				},
				set(val) { 
					if (typeof this.setAttribute === 'function') {
						this.setAttribute('href', val); 
					}
				},
				configurable: true,
				enumerable: false
			});
			
			Object.defineProperty(proto, 'src', {
				get() { 
					if (typeof this.getAttribute === 'function') {
						return this.getAttribute('src'); 
					}
					return this.Src || '';
				},
				set(val) { 
					if (typeof this.setAttribute === 'function') {
						this.setAttribute('src', val); 
					}
				},
				configurable: true,
				enumerable: false
			});

			globalThis.Worker = function() {
				this.postMessage = function() {};
				this.terminate = function() {};
			};
			globalThis.ServiceWorker = function() {};
			
			globalThis.crypto = {
				getRandomValues: function(arr) { return arr; },
				subtle: {
					digest: function() { return Promise.resolve(new ArrayBuffer()); },
					encrypt: function() { return Promise.resolve(new ArrayBuffer()); },
					decrypt: function() { return Promise.resolve(new ArrayBuffer()); },
					sign: function() { return Promise.resolve(new ArrayBuffer()); },
					verify: function() { return Promise.resolve(true); }
				}
			};

			globalThis.indexedDB = {
				open: function() { return { onupgradeneeded: null, onsuccess: null, onerror: null }; },
				deleteDatabase: function() { return { onsuccess: null, onerror: null }; }
			};

			globalThis.CustomEvent = function(type, params) {
				const e = new globalThis.Event(type);
				if (params) e.detail = params.detail;
				return e;
			};
			globalThis.IntersectionObserver = class IntersectionObserver {
				constructor(callback, options) {
					this.callback = callback;
				}
				observe(target) {
					if (this.callback) {
						Promise.resolve().then(() => {
							this.callback([{ target: target, isIntersecting: true, intersectionRatio: 1.0 }]);
						});
					}
				}
				unobserve(target) {}
				disconnect() {}
			};

			globalThis.ResizeObserver = class ResizeObserver {
				constructor(callback) {}
				observe(target) {}
				unobserve(target) {}
				disconnect() {}
			};

			globalThis.MutationObserver = class MutationObserver {
				constructor(callback) {}
				observe(target, options) {}
				disconnect() {}
				takeRecords() { return []; }
			};

			globalThis.history = {
				pushState: function() {},
				replaceState: function() {},
				go: function() {},
				back: function() {},
				forward: function() {},
				length: 1
			};
			globalThis.URLSearchParams = class URLSearchParams {
				constructor(init) {
					this._params = new Map();
					if (typeof init === 'string') {
						if (init.startsWith('?')) init = init.slice(1);
						const pairs = init.split('&');
						for (const p of pairs) {
							if (!p) continue;
							const idx = p.indexOf('=');
							if (idx === -1) {
								this.append(decodeURIComponent(p), '');
							} else {
								this.append(decodeURIComponent(p.slice(0, idx)), decodeURIComponent(p.slice(idx+1)));
							}
						}
					}
				}
				append(name, value) {
					if (!this._params.has(name)) this._params.set(name, []);
					this._params.get(name).push(value);
				}
				get(name) {
					const vals = this._params.get(name);
					return vals ? vals[0] : null;
				}
				getAll(name) {
					return this._params.get(name) || [];
				}
				has(name) {
					return this._params.has(name);
				}
				set(name, value) {
					this._params.set(name, [value]);
				}
				delete(name) {
					this._params.delete(name);
				}
				toString() {
					const parts = [];
					for (const [k, v] of this._params) {
						for (const val of v) {
							parts.push(encodeURIComponent(k) + '=' + encodeURIComponent(val));
						}
					}
					return parts.join('&');
				}
			};

			Object.defineProperty(proto, 'body', {
				get() {
					if (Number(this.nodeType) === 9) { // DocumentNode
						if (typeof this.getBody === 'function') {
							return this.getBody();
						}
					}
					return undefined;
				},
				set(val) {},
				configurable: true,
				enumerable: false
			});

			Object.defineProperty(proto, 'head', {
				get() {
					if (Number(this.nodeType) === 9) { // DocumentNode
						if (typeof this.getHead === 'function') {
							return this.getHead();
						}
					}
					return undefined;
				},
				set(val) {},
				configurable: true,
				enumerable: false
			});

			Object.defineProperty(proto, 'documentElement', {
				get() {
					if (Number(this.nodeType) === 9) { // DocumentNode
						if (typeof this.getDocumentElement === 'function') {
							return this.getDocumentElement();
						}
					}
					return undefined;
				},
				set(val) {},
				configurable: true,
				enumerable: false
			});

			proto.write = function(markup) {
				const body = document.body || document;
				const temp = document.createElement('div');
				temp.innerHTML = markup;
				while (temp.childNodes && temp.childNodes.length > 0) {
					body.appendChild(temp.childNodes[0]);
				}
			};

			// Bind length getter to the Storage prototype
			if (typeof localStorage !== 'undefined') {
				const storageProto = Object.getPrototypeOf(localStorage);
				if (storageProto) {
					Object.defineProperty(storageProto, 'length', {
						get() { 
							if (typeof this.getLength === 'function') {
								return this.getLength(); 
							}
							return 0;
						},
						configurable: true,
						enumerable: false
					});
				}
			}
		})();
	`
	_, _ = vm.RunString(setupScript)
}
