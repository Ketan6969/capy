package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Ketan6969/capy/internal/dom"
	"github.com/Ketan6969/capy/internal/engine"
)

func bootstrapPageScripts(runCtx *engine.Context, doc *dom.Node, baseRef string) error {
	if doc == nil {
		return nil
	}

	scripts := doc.GetElementsByTagName("script")
	for i, scriptNode := range scripts {
		if !isExecutableScript(scriptNode.GetAttribute("type")) {
			continue
		}

		src := strings.TrimSpace(scriptNode.GetAttribute("src"))
		scriptName := fmt.Sprintf("inline-script-%d.js", i+1)
		content := strings.TrimSpace(stripCDATA(scriptNode.GetTextContent()))

		if src != "" {
			resolved, err := resolveScriptRef(baseRef, src)
			if err != nil {
				return fmt.Errorf("resolve script %q: %w", src, err)
			}

			body, err := loadHTML(resolved)
			if err != nil {
				return fmt.Errorf("load script %q: %w", resolved, err)
			}
			if strings.HasPrefix(strings.TrimSpace(body), "<") {
				continue
			}

			scriptName = resolved
			content = stripCDATA(body)
		}

		if strings.TrimSpace(content) == "" {
			continue
		}

		runCtx.RunScript(scriptName, content)
	}

	return nil
}

func isExecutableScript(scriptType string) bool {
	switch strings.ToLower(strings.TrimSpace(scriptType)) {
	case "", "text/javascript", "application/javascript", "text/ecmascript", "module":
		return true
	default:
		return false
	}
}

func stripCDATA(text string) string {
	replacer := strings.NewReplacer(
		"/* <![CDATA[ */", "",
		"/* ]]> */", "",
		"//<![CDATA[", "",
		"//]]>", "",
	)
	return replacer.Replace(text)
}

func resolveScriptRef(baseRef, scriptRef string) (string, error) {
	if strings.HasPrefix(scriptRef, "http://") || strings.HasPrefix(scriptRef, "https://") {
		return scriptRef, nil
	}

	if strings.HasPrefix(baseRef, "http://") || strings.HasPrefix(baseRef, "https://") {
		baseURL, err := url.Parse(baseRef)
		if err != nil {
			return "", err
		}
		refURL, err := url.Parse(scriptRef)
		if err != nil {
			return "", err
		}
		return baseURL.ResolveReference(refURL).String(), nil
	}

	basePath := baseRef
	if basePath == "" {
		basePath = "."
	}
	if strings.HasPrefix(scriptRef, "/") {
		return scriptRef, nil
	}
	if info, err := os.Stat(basePath); err == nil && !info.IsDir() {
		basePath = filepath.Dir(basePath)
	} else if filepath.Ext(basePath) != "" {
		basePath = filepath.Dir(basePath)
	}
	return filepath.Clean(filepath.Join(basePath, scriptRef)), nil
}
