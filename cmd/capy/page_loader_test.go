package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Ketan6969/capy/internal/dom"
	"github.com/Ketan6969/capy/internal/engine"
	"github.com/Ketan6969/capy/internal/network"
)

func TestBootstrapPageScriptsLoadsLocalScripts(t *testing.T) {
	tempDir := t.TempDir()
	htmlPath := filepath.Join(tempDir, "index.html")
	scriptPath := filepath.Join(tempDir, "app.js")

	htmlContent := `<!DOCTYPE html><html><body><div id="app">before</div><script src="app.js"></script></body></html>`
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("write html: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte(`document.querySelector("#app").textContent = "after";`), 0644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	doc, err := dom.ParseHTML(htmlContent)
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	runCtx := engine.NewContext(context.Background())
	defer runCtx.Close()

	nm := network.NewNetworkManager()
	dom.SetupDOM(runCtx.VM(), doc, htmlPath)
	dom.SetupCookies(runCtx.VM(), nm.CookieJar(), htmlPath)

	if err := bootstrapPageScripts(runCtx, doc, htmlPath); err != nil {
		t.Fatalf("bootstrap scripts: %v", err)
	}
	if err := runCtx.EventLoop(); err != nil {
		t.Fatalf("event loop: %v", err)
	}

	app := doc.QuerySelector("#app")
	if app == nil {
		t.Fatal("app node not found")
	}
	if got := app.GetTextContent(); got != "after" {
		t.Fatalf("expected script to update DOM to %q, got %q", "after", got)
	}
}

func TestResolveScriptRefForLocalFile(t *testing.T) {
	resolved, err := resolveScriptRef("/tmp/site/index.html", "assets/app.js")
	if err != nil {
		t.Fatalf("resolve script ref: %v", err)
	}
	if resolved != filepath.Clean("/tmp/site/assets/app.js") {
		t.Fatalf("unexpected resolved path: %s", resolved)
	}
}
