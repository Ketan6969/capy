package engine

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja/parser"
)

// Engine represents the global environment manager.
type Engine struct{}

// NewEngine creates a new Engine instance.
func NewEngine() *Engine {
	return &Engine{}
}

// Context wraps a Goja runtime and coordinates its single-threaded execution,
// event loop, and web api bindings.
type Context struct {
	vm     *goja.Runtime
	jobs   chan func()
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
	closed bool
	err    error
	parent *Context

	// Timer management
	timerMu sync.Mutex
	timerId int
	timers  map[int]func()

	// Optional collector for non-fatal JS errors (used by audit)
	errorCollector func(msg string)
}

// NewContext creates a new execution context.
func NewContext(ctx context.Context) *Context {
	ctx, cancel := context.WithCancel(ctx)
	vm := goja.New()
	// Disable source map loading: the default loader reads from the local filesystem,
	// which produces noisy file-not-found errors for every minified remote script.
	vm.SetParserOptions(parser.WithDisableSourceMaps)
	vm.SetMaxCallStackSize(10000)
	c := &Context{
		vm:     vm,
		ctx:    ctx,
		cancel: cancel,
		jobs:   make(chan func(), 1024),
		timers: make(map[int]func()),
	}

	go func() {
		<-ctx.Done()
		vm.Interrupt(ctx.Err())
	}()

	c.vm.SetFieldNameMapper(goja.UncapFieldNameMapper())
	c.setupBasicPrimitives()
	SetupWorkers(c)
	return c
}

// VM returns the underlying Goja VM.
// WARNING: Goja is NOT thread-safe. Only interact with VM properties
// inside jobs running on the event loop.
func (c *Context) VM() *goja.Runtime {
	return c.vm
}

// RunScript schedules a script execution in the context's event loop.
func (c *Context) RunScript(name, content string) {
	c.WgAdd(1)
	slog.Debug("Enqueuing script", "name", name)
	c.Enqueue(func() {
		defer c.WgDone()
		slog.Debug("Executing script", "name", name)
		_, err := c.vm.RunScript(name, content)
		if err != nil {
			slog.Error("Script execution failed", "name", name, "error", err)
			c.mu.Lock()
			hasCollector := c.errorCollector != nil
			c.mu.Unlock()
			c.setError(err)
			// In audit mode (collector is set), don't cancel the context —
			// we want to survive individual script failures and keep running.
			if !hasCollector {
				c.cancel()
			}
		}
	})
}

// Enqueue adds a job to the execution queue.
func (c *Context) Enqueue(job func()) {
	select {
	case c.jobs <- job:
	case <-c.ctx.Done():
	}
}

// WgAdd increments the WaitGroup counter.
func (c *Context) WgAdd(delta int) {
	c.wg.Add(delta)
	c.mu.Lock()
	p := c.parent
	c.mu.Unlock()
	if p != nil {
		p.WgAdd(delta)
	}
}

// WgDone decrements the WaitGroup counter.
func (c *Context) WgDone() {
	c.wg.Done()
	c.mu.Lock()
	p := c.parent
	c.mu.Unlock()
	if p != nil {
		p.WgDone()
	}
}

// SetError sets the execution error.
func (c *Context) SetError(err error) {
	c.setError(err)
}

// Cancel cancels the context.
func (c *Context) Cancel() {
	c.cancel()
}

// Ctx returns the underlying context.Context.
func (c *Context) Ctx() context.Context {
	return c.ctx
}

// SetErrorCollector registers a callback that receives non-fatal JS error messages.
// When set, errors are forwarded to the collector instead of terminating the run.
func (c *Context) SetErrorCollector(collector func(msg string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.errorCollector = collector
}

func (c *Context) setError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.errorCollector != nil {
		// In audit/collect mode: forward the error message and don't terminate.
		c.errorCollector(err.Error())
		return
	}
	if c.err == nil {
		c.err = err
	}
}

// EventLoop runs until all tasks finish or the context is cancelled.
func (c *Context) EventLoop() error {
	c.mu.Lock()
	isWorker := c.parent != nil
	c.mu.Unlock()

	if isWorker {
		// Worker EventLoop runs until explicit cancellation (Close or parent context cancellation)
		for {
			select {
			case job := <-c.jobs:
				job()
			case <-c.ctx.Done():
				c.mu.Lock()
				if c.err == nil {
					c.err = c.ctx.Err()
				}
				err := c.err
				c.mu.Unlock()
				return err
			}
		}
	}

	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	for {
		select {
		case job := <-c.jobs:
			job()
		case <-done:
			c.mu.Lock()
			err := c.err
			c.mu.Unlock()
			return err
		case <-c.ctx.Done():
			c.mu.Lock()
			if c.err == nil {
				c.err = c.ctx.Err()
			}
			err := c.err
			c.mu.Unlock()
			return err
		}
	}
}

// Close terminates execution and releases associated resources.
func (c *Context) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		c.cancel()

		// Cancel all active timers
		c.timerMu.Lock()
		for _, cancelTimer := range c.timers {
			cancelTimer()
		}
		c.timers = nil
		c.timerMu.Unlock()
	}
}

func (c *Context) setupBasicPrimitives() {
	// 1. Setup Console
	console := c.vm.NewObject()
	console.Set("log", func(call goja.FunctionCall) goja.Value {
		args := make([]interface{}, len(call.Arguments))
		for i, arg := range call.Arguments {
			args[i] = arg.Export()
		}
		fmt.Println(args...)
		return goja.Undefined()
	})
	console.Set("error", func(call goja.FunctionCall) goja.Value {
		args := make([]interface{}, len(call.Arguments))
		for i, arg := range call.Arguments {
			args[i] = arg.Export()
		}
		fmt.Print("ERROR: ")
		fmt.Println(args...)
		return goja.Undefined()
	})
	c.vm.Set("console", console)

	c.vm.Set("_goGetMemoryUsage", func(call goja.FunctionCall) goja.Value {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		obj := c.vm.NewObject()
		_ = obj.Set("alloc", m.Alloc)
		_ = obj.Set("totalAlloc", m.TotalAlloc)
		_ = obj.Set("sys", m.Sys)
		_ = obj.Set("heapAlloc", m.HeapAlloc)
		return obj
	})

	// 2. Setup setTimeout & clearTimeout
	c.vm.Set("setTimeout", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(c.vm.NewTypeError("setTimeout: callback required"))
		}
		callback, ok := goja.AssertFunction(call.Arguments[0])
		if !ok {
			panic(c.vm.NewTypeError("setTimeout: first argument must be a function"))
		}

		delayMs := int64(0)
		if len(call.Arguments) > 1 {
			delayMs = call.Arguments[1].ToInteger()
		}

		c.timerMu.Lock()
		c.timerId++
		id := c.timerId
		timerCtx, cancelTimer := context.WithCancel(c.ctx)
		c.timers[id] = cancelTimer
		c.timerMu.Unlock()

		c.WgAdd(1)
		go func() {
			select {
			case <-time.After(time.Duration(delayMs) * time.Millisecond):
				c.Enqueue(func() {
					defer c.WgDone()
					c.timerMu.Lock()
					_, active := c.timers[id]
					if active {
						delete(c.timers, id)
					}
					c.timerMu.Unlock()
					if !active {
						return
					}

					var cbArgs []goja.Value
					if len(call.Arguments) > 2 {
						cbArgs = call.Arguments[2:]
					}
					_, err := callback(goja.Undefined(), cbArgs...)
					if err != nil {
						c.setError(err)
						c.cancel()
					}
				})
			case <-timerCtx.Done():
				c.timerMu.Lock()
				if c.timers != nil {
					delete(c.timers, id)
				}
				c.timerMu.Unlock()
				c.WgDone()
			}
		}()

		return c.vm.ToValue(id)
	})

	c.vm.Set("clearTimeout", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		id := int(call.Arguments[0].ToInteger())
		c.timerMu.Lock()
		if cancelTimer, ok := c.timers[id]; ok {
			cancelTimer()
			delete(c.timers, id)
		}
		c.timerMu.Unlock()
		return goja.Undefined()
	})

	// 3. Setup setInterval & clearInterval
	c.vm.Set("setInterval", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(c.vm.NewTypeError("setInterval: callback required"))
		}
		callback, ok := goja.AssertFunction(call.Arguments[0])
		if !ok {
			panic(c.vm.NewTypeError("setInterval: first argument must be a function"))
		}

		delayMs := int64(0)
		if len(call.Arguments) > 1 {
			delayMs = call.Arguments[1].ToInteger()
		}
		if delayMs < 1 {
			delayMs = 1 // Prevent infinite spin/instant triggers
		}

		c.timerMu.Lock()
		c.timerId++
		id := c.timerId
		timerCtx, cancelTimer := context.WithCancel(c.ctx)
		c.timers[id] = cancelTimer
		c.timerMu.Unlock()

		c.WgAdd(1)
		go func() {
			defer func() {
				c.timerMu.Lock()
				if c.timers != nil {
					delete(c.timers, id)
				}
				c.timerMu.Unlock()
				c.WgDone()
			}()

			ticker := time.NewTicker(time.Duration(delayMs) * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					// Run the callback inside the event loop
					doneChan := make(chan struct{})
					c.Enqueue(func() {
						defer close(doneChan)
						c.timerMu.Lock()
						_, active := c.timers[id]
						c.timerMu.Unlock()
						if !active {
							return
						}

						var cbArgs []goja.Value
						if len(call.Arguments) > 2 {
							cbArgs = call.Arguments[2:]
						}
						_, err := callback(goja.Undefined(), cbArgs...)
						if err != nil {
							c.setError(err)
							c.cancel()
						}
					})
					// Wait for the job to run in event loop before ticking again to avoid piling up ticks
					select {
					case <-doneChan:
					case <-timerCtx.Done():
						return
					}
				case <-timerCtx.Done():
					return
				}
			}
		}()

		return c.vm.ToValue(id)
	})

	c.vm.Set("clearInterval", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		id := int(call.Arguments[0].ToInteger())
		c.timerMu.Lock()
		if cancelTimer, ok := c.timers[id]; ok {
			cancelTimer()
			delete(c.timers, id)
		}
		c.timerMu.Unlock()
		return goja.Undefined()
	})
}
