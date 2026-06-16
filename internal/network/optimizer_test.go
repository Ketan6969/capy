package network

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Ketan6969/capy/internal/engine"
)

func TestOptimizerRecordAndReplay(t *testing.T) {
	// 1. Setup mock server
	var requestCount int
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom-Header", "test-val")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"msg": "hello from server", "count": %d}`, requestCount)))
	}))
	defer mockServer.Close()

	// Temp file to save rules
	tempDir := t.TempDir()
	rulesPath := filepath.Join(tempDir, "rules.json")

	// 2. RUN RECORD
	{
		ctx := engine.NewContext(context.Background())
		nm := NewNetworkManager()
		nm.Optimizer.SetRecordMode(true)
		nm.SetupNetwork(ctx)

		// Set mock server URL in JS
		ctx.VM().Set("SERVER_URL", mockServer.URL)

		ctx.RunScript("record.js", `
			fetch(SERVER_URL, {
				method: 'POST',
				headers: {
					'Content-Type': 'text/plain',
					'X-Req-Header': 'hello'
				},
				body: 'some request body'
			});
		`)

		err := ctx.EventLoop()
		if err != nil {
			t.Fatalf("EventLoop failed in record: %v", err)
		}

		ctx.Close()

		// Save the rules
		err = nm.Optimizer.Save(rulesPath)
		if err != nil {
			t.Fatalf("failed to save rules: %v", err)
		}
	}

	// Verify the file was written
	if _, err := os.Stat(rulesPath); err != nil {
		t.Fatalf("rules file not found: %v", err)
	}

	// 3. RUN REPLAY
	{
		nm := NewNetworkManager()
		replayBytes, err := nm.Optimizer.Replay(rulesPath)
		if err != nil {
			t.Fatalf("Replay failed: %v", err)
		}

		var replayRes ReplayResult
		if err := json.Unmarshal(replayBytes, &replayRes); err != nil {
			t.Fatalf("failed to unmarshal replay results: %v", err)
		}

		if len(replayRes.Results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(replayRes.Results))
		}

		res := replayRes.Results[0]
		if res.URL != mockServer.URL {
			t.Errorf("expected URL %s, got %s", mockServer.URL, res.URL)
		}
		if res.Status != 200 {
			t.Errorf("expected status 200, got %d", res.Status)
		}
		if !json.Valid([]byte(res.Body)) {
			t.Errorf("expected valid JSON body, got %s", res.Body)
		}
		if res.Headers["Content-Type"] != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", res.Headers["Content-Type"])
		}
		if res.Headers["X-Custom-Header"] != "test-val" {
			t.Errorf("expected X-Custom-Header test-val, got %s", res.Headers["X-Custom-Header"])
		}
	}

	// Verify server got two requests (one record, one replay)
	if requestCount != 2 {
		t.Errorf("expected 2 total requests to server, got %d", requestCount)
	}
}
