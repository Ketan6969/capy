package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Ketan6969/capy/internal/dom"
	"github.com/Ketan6969/capy/internal/engine"
	"github.com/Ketan6969/capy/internal/network"
)

type ExtractRequest struct {
	URL    string `json:"url"`
	Script string `json:"script"`
}

type ExtractResponse struct {
	Result        string `json:"result"`
	Error         string `json:"error,omitempty"`
	ExecutionTime string `json:"executionTime"`
}

// Global concurrency limit for /extract requests to prevent OOM.
// 10 concurrent requests allows high throughput while keeping memory bounded.
var extractSemaphore = make(chan struct{}, 10)

func (app *serverApp) handleExtract(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Wait for a slot to execute
	select {
	case extractSemaphore <- struct{}{}:
		defer func() { <-extractSemaphore }()
	case <-r.Context().Done():
		http.Error(w, "Request cancelled while waiting in queue", http.StatusRequestTimeout)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req ExtractRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateExtractRequest(req.URL, req.Script); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	start := time.Now()

	html, err := loadHTML(req.URL)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ExtractResponse{
			Error:         err.Error(),
			ExecutionTime: fmt.Sprintf("%.3fs", time.Since(start).Seconds()),
		})
		return
	}

	bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	runCtx := engine.NewContext(bgCtx)
	defer runCtx.Close()

	nm := network.NewNetworkManager()
	doc, _ := dom.ParseHTML(html)
	dom.SetupDOM(runCtx.VM(), doc, req.URL)
	dom.SetupCookies(runCtx.VM(), nm.CookieJar(), req.URL)

	nm.SetupNetwork(runCtx)

	if err := bootstrapPageScripts(runCtx, doc, req.URL); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ExtractResponse{
			Error:         err.Error(),
			ExecutionTime: fmt.Sprintf("%.3fs", time.Since(start).Seconds()),
		})
		return
	}

	dom.DispatchLifecycleEvents(runCtx.VM())

	runCtx.RunScript("extract.js", fmt.Sprintf(`
		globalThis.__extract_result__ = (function() {
			try {
				%s
			} catch(e) {
				return e.toString();
			}
		})();
	`, req.Script))

	err = runCtx.EventLoop()
	var resultStr string
	if err != nil {
		resultStr = err.Error()
	} else {
		val := runCtx.VM().Get("__extract_result__")
		if val != nil {
			resultStr = fmt.Sprintf("%v", val.Export())
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ExtractResponse{
		Result:        resultStr,
		ExecutionTime: fmt.Sprintf("%.3fs", time.Since(start).Seconds()),
	})
}

func startServer(port int) {
	cfg := loadServerConfig()
	handler := newServerHandler(cfg)

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("Capy API Server listening on %s\n", addr)

	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      45 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		fmt.Printf("Server failed: %v\n", err)
	}
}
