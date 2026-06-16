package capy

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/Ketan6969/capy/internal/dom"
	"github.com/Ketan6969/capy/internal/engine"
	"github.com/Ketan6969/capy/internal/network"
)

// Capy represents a single execution environment.
type Capy struct {
	ctx *engine.Context
	doc *dom.Node
}

// New creates a new capy environment tied to the provided context.
func New(ctx context.Context) *Capy {
	runCtx := engine.NewContext(ctx)
	
	// Setup core features
	networkMgr := network.NewNetworkManager()
	networkMgr.SetupNetwork(runCtx)
	engine.SetupWorkers(runCtx)

	return &Capy{
		ctx: runCtx,
	}
}

// LoadHTML loads a raw HTML string into the environment.
func (b *Capy) LoadHTML(htmlStr string) error {
	documentRoot, err := dom.ParseHTML(htmlStr)
	if err != nil {
		return err
	}
	b.doc = documentRoot
	dom.SetupDOM(b.ctx.VM(), documentRoot, "http://localhost")
	dom.DispatchLifecycleEvents(b.ctx.VM())
	return nil
}

// LoadURL fetches a URL and loads it into the environment.
// It also executes any synchronous <script> tags found in the HTML.
func (b *Capy) LoadURL(urlStr string) error {
	req, err := http.NewRequestWithContext(b.ctx.Ctx(), "GET", urlStr, nil)
	if err != nil {
		return err
	}
	// Use standard headers to pretend we are a browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("non-200 status code: " + resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	documentRoot, err := dom.ParseHTML(string(body))
	if err != nil {
		return err
	}

	b.doc = documentRoot
	dom.SetupDOM(b.ctx.VM(), documentRoot, urlStr)
	
	// Expose a basic fetch loop for script tags
	scripts := documentRoot.GetElementsByTagName("script")
	for _, script := range scripts {
		src := script.GetAttribute("src")
		if src != "" {
			// Resolve URL
			resolved := b.resolveURL(urlStr, src)
			b.ctx.RunScript(src, "fetch('"+resolved+"').then(r => r.text()).then(eval).catch(console.error);")
		} else {
			code := script.GetInnerHTML()
			b.ctx.RunScript("inline", code)
		}
	}

	dom.DispatchLifecycleEvents(b.ctx.VM())
	
	// Wait for network requests and event loop to finish
	return b.ctx.EventLoop()
}

// Evaluate runs custom JavaScript inside the environment.
func (b *Capy) Evaluate(script string) error {
	b.ctx.RunScript("eval", script)
	return b.ctx.EventLoop()
}

// Close terminates the environment and releases resources.
func (b *Capy) Close() {
	b.ctx.Close()
}

// Document returns the parsed HTML document root, allowing for native Go DOM querying.
func (b *Capy) Document() *dom.Node {
	return b.doc
}

// Click natively dispatches a click event on the target element inside the JavaScript environment.
func (b *Capy) Click(node *dom.Node) error {
	vm := b.ctx.VM()
	vm.Set("__capyTargetNode", node)
	script := `
		if (typeof __capyTargetNode !== 'undefined' && __capyTargetNode !== null) {
			__capyTargetNode.dispatchEvent(new Event('click', { bubbles: true, cancelable: true }));
			if (typeof __capyTargetNode.click === 'function') {
				__capyTargetNode.click();
			}
		}
	`
	b.ctx.RunScript("click", script)
	return b.ctx.EventLoop()
}

// Type natively focuses the element and types text inside the JavaScript environment.
func (b *Capy) Type(node *dom.Node, text string) error {
	vm := b.ctx.VM()
	vm.Set("__capyTargetNode", node)
	vm.Set("__capyTargetText", text)
	script := `
		if (typeof __capyTargetNode !== 'undefined' && __capyTargetNode !== null) {
			if (typeof __capyTargetNode.focus === 'function') __capyTargetNode.focus();
			__capyTargetNode.value = __capyTargetText;
			__capyTargetNode.dispatchEvent(new Event('input', { bubbles: true }));
			__capyTargetNode.dispatchEvent(new Event('change', { bubbles: true }));
		}
	`
	b.ctx.RunScript("type", script)
	return b.ctx.EventLoop()
}

func (b *Capy) resolveURL(base, ref string) string {
	l := dom.NewLocation(base)
	return l.ResolveURL(ref)
}
