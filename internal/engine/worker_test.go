package engine

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/dop251/goja"
)

func TestWebWorkerCommunication(t *testing.T) {
	tempDir := t.TempDir()
	workerScriptPath := filepath.Join(tempDir, "worker.js")

	workerContent := `
		self.onmessage = function(e) {
			const data = e.data;
			postMessage(data + 10);
		};
	`
	err := os.WriteFile(workerScriptPath, []byte(workerContent), 0644)
	if err != nil {
		t.Fatalf("failed to write worker script: %v", err)
	}

	ctx := NewContext(context.Background())
	defer ctx.Close()

	var outputVal interface{}
	var mu sync.Mutex
	ctx.vm.Set("saveResult", func(call goja.FunctionCall) goja.Value {
		mu.Lock()
		defer mu.Unlock()
		if len(call.Arguments) > 0 {
			outputVal = call.Arguments[0].Export()
		}
		return goja.Undefined()
	})

	// Inject the script path as a JS global variable
	ctx.vm.Set("WORKER_PATH", workerScriptPath)

	ctx.RunScript("main.js", `
		const w = new Worker(WORKER_PATH);
		w.onmessage = function(e) {
			saveResult(e.data);
		};
		w.postMessage(42);
	`)

	err = ctx.EventLoop()
	if err != nil {
		t.Fatalf("EventLoop failed: %v", err)
	}

	mu.Lock()
	res := outputVal
	mu.Unlock()

	if res == nil {
		t.Fatal("expected message callback from worker, but result was nil")
	}

	// 42 + 10 = 52
	if val, ok := res.(int64); !ok || val != 52 {
		t.Errorf("expected 52, got %v (%T)", res, res)
	}
}

func TestWebWorkerTermination(t *testing.T) {
	tempDir := t.TempDir()
	workerScriptPath := filepath.Join(tempDir, "worker_loop.js")

	workerContent := `
		setInterval(() => {
			postMessage("tick");
		}, 5);
	`
	err := os.WriteFile(workerScriptPath, []byte(workerContent), 0644)
	if err != nil {
		t.Fatalf("failed to write worker script: %v", err)
	}

	ctx := NewContext(context.Background())
	defer ctx.Close()

	var tickCount int
	var mu sync.Mutex
	ctx.vm.Set("onTick", func(call goja.FunctionCall) goja.Value {
		mu.Lock()
		tickCount++
		mu.Unlock()
		return goja.Undefined()
	})

	ctx.vm.Set("WORKER_PATH", workerScriptPath)

	ctx.RunScript("main.js", `
		var w = new Worker(WORKER_PATH);
		w.onmessage = function(e) {
			onTick();
			w.terminate();
		};
	`)

	err = ctx.EventLoop()
	if err != nil {
		t.Fatalf("EventLoop failed: %v", err)
	}

	mu.Lock()
	finalTicks := tickCount
	mu.Unlock()

	if finalTicks != 1 {
		t.Errorf("expected exactly 1 tick before termination, got %d", finalTicks)
	}
}
