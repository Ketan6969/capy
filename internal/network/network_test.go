package network

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Ketan6969/capy/internal/engine"
)

func TestFetch(t *testing.T) {
	// 1. Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			body, _ := io.ReadAll(r.Body)
			var data map[string]interface{}
			_ = json.Unmarshal(body, &data)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"received": true, "method": "POST", "name": "%s"}`, data["name"])))
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("X-Custom-Header", "HelloTest")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Hello from local test server!"))
	}))
	defer server.Close()

	// 2. Initialize engine and context
	ctx := engine.NewContext(context.Background())
	defer ctx.Close()

	nm := NewNetworkManager()
	nm.SetupNetwork(ctx)

	// Bind results to check from Go
	var getResult string
	var postResultName string
	var responseHeaderVal string
	var status int

	ctx.VM().Set("saveGetResult", func(val string) {
		getResult = val
	})
	ctx.VM().Set("savePostResult", func(val string) {
		postResultName = val
	})
	ctx.VM().Set("saveHeaderVal", func(val string) {
		responseHeaderVal = val
	})
	ctx.VM().Set("saveStatus", func(val int) {
		status = val
	})

	// 3. Run script performing GET and POST
	script := fmt.Sprintf(`
		// Test GET
		fetch(%q)
			.then(res => {
				saveStatus(res.status);
				saveHeaderVal(res.headers.get("X-Custom-Header"));
				return res.text();
			})
			.then(text => {
				saveGetResult(text);
				
				// Test POST
				return fetch(%q, {
					method: "POST",
					headers: { "Content-Type": "application/json" },
					body: JSON.stringify({ name: "Capy" })
				});
			})
			.then(res => res.json())
			.then(data => {
				savePostResult(data.name);
			});
	`, server.URL, server.URL)

	ctx.RunScript("test.js", script)

	err := ctx.EventLoop()
	if err != nil {
		t.Fatalf("EventLoop failed: %v", err)
	}

	// 4. Assertions
	if status != 200 {
		t.Errorf("Expected status 200, got: %d", status)
	}
	if responseHeaderVal != "HelloTest" {
		t.Errorf("Expected response header 'HelloTest', got: %s", responseHeaderVal)
	}
	if getResult != "Hello from local test server!" {
		t.Errorf("Expected GET result 'Hello from local test server!', got: %s", getResult)
	}
	if postResultName != "Capy" {
		t.Errorf("Expected POST result name 'Capy', got: %s", postResultName)
	}
}

func TestFetchCookies(t *testing.T) {
	// 1. Create a test HTTP server that sets a cookie on /set and checks it on /check
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/set" {
			http.SetCookie(w, &http.Cookie{
				Name:  "session_id",
				Value: "12345",
				Path:  "/",
			})
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Cookie set"))
			return
		}

		if r.URL.Path == "/check" {
			cookie, err := r.Cookie("session_id")
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("No cookie found"))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(fmt.Sprintf("Cookie value: %s", cookie.Value)))
			return
		}
	}))
	defer server.Close()

	ctx := engine.NewContext(context.Background())
	defer ctx.Close()

	nm := NewNetworkManager()
	nm.SetupNetwork(ctx)

	var checkResult string
	ctx.VM().Set("saveCheckResult", func(val string) {
		checkResult = val
	})

	script := fmt.Sprintf(`
		fetch(%q + "/set")
			.then(res => fetch(%q + "/check"))
			.then(res => res.text())
			.then(text => saveCheckResult(text));
	`, server.URL, server.URL)

	ctx.RunScript("test.js", script)

	err := ctx.EventLoop()
	if err != nil {
		t.Fatalf("EventLoop failed: %v", err)
	}

	if checkResult != "Cookie value: 12345" {
		t.Errorf("Expected cookie check value 'Cookie value: 12345', got '%s'", checkResult)
	}
}
