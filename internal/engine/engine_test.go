package engine

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dop251/goja"
)

func TestContextBasicRun(t *testing.T) {
	ctx := NewContext(context.Background())
	defer ctx.Close()

	var output []string
	var mu sync.Mutex

	// Mock console.log
	ctx.vm.Set("console", map[string]interface{}{
		"log": func(call goja.FunctionCall) goja.Value {
			mu.Lock()
			defer mu.Unlock()
			for _, arg := range call.Arguments {
				output = append(output, arg.String())
			}
			return goja.Undefined()
		},
	})

	ctx.RunScript("test.js", `
		const x = 10;
		console.log("value of x is", x);
	`)

	err := ctx.EventLoop()
	if err != nil {
		t.Fatalf("EventLoop failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	joined := strings.Join(output, " ")
	if !strings.Contains(joined, "value of x is 10") {
		t.Errorf("Expected console.log output, got: %s", joined)
	}
}

func TestContextTimeout(t *testing.T) {
	ctx := NewContext(context.Background())
	defer ctx.Close()

	var triggered bool
	ctx.vm.Set("markTriggered", func(call goja.FunctionCall) goja.Value {
		triggered = true
		return goja.Undefined()
	})

	ctx.RunScript("test.js", `
		setTimeout(() => {
			markTriggered();
		}, 10);
	`)

	err := ctx.EventLoop()
	if err != nil {
		t.Fatalf("EventLoop failed: %v", err)
	}

	if !triggered {
		t.Error("Expected setTimeout callback to be triggered, but it was not")
	}
}

func TestContextClearTimeout(t *testing.T) {
	ctx := NewContext(context.Background())
	defer ctx.Close()

	var triggered bool
	ctx.vm.Set("markTriggered", func(call goja.FunctionCall) goja.Value {
		triggered = true
		return goja.Undefined()
	})

	ctx.RunScript("test.js", `
		const id = setTimeout(() => {
			markTriggered();
		}, 10);
		clearTimeout(id);
	`)

	// Let the event loop run. We sleep a bit first to ensure the goroutine could have run
	// if it wasn't cancelled.
	go func() {
		time.Sleep(50 * time.Millisecond)
	}()

	err := ctx.EventLoop()
	if err != nil {
		t.Fatalf("EventLoop failed: %v", err)
	}

	if triggered {
		t.Error("Expected setTimeout to be cleared and not trigger callback")
	}
}

func TestContextInterval(t *testing.T) {
	ctx := NewContext(context.Background())
	defer ctx.Close()

	var count int
	ctx.vm.Set("increment", func(call goja.FunctionCall) goja.Value {
		count++
		return goja.Undefined()
	})

	ctx.RunScript("test.js", `
		let jsCount = 0;
		const id = setInterval(() => {
			jsCount++;
			increment();
			if (jsCount === 3) {
				clearInterval(id);
			}
		}, 5);
	`)

	err := ctx.EventLoop()
	if err != nil {
		t.Fatalf("EventLoop failed: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected interval to trigger exactly 3 times, got: %d", count)
	}
}
