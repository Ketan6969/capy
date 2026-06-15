package engine

import (
	"fmt"
	"os"
	"sync"

	"github.com/dop251/goja"
)

// WorkerRegistry manages all background Web Worker contexts spawned by a parent context.
type WorkerRegistry struct {
	mu        sync.Mutex
	workers   map[int]*Context
	callbacks map[int]goja.Value
	idSeq     int
	parent    *Context
}

// NewWorkerRegistry creates a new WorkerRegistry.
func NewWorkerRegistry(parent *Context) *WorkerRegistry {
	return &WorkerRegistry{
		workers:   make(map[int]*Context),
		callbacks: make(map[int]goja.Value),
		parent:    parent,
	}
}

// SpawnWorker initializes and runs a background worker script in a separate context.
func (wr *WorkerRegistry) SpawnWorker(scriptPath string, onMessageCallback goja.Value) int {
	wr.mu.Lock()
	wr.idSeq++
	id := wr.idSeq
	wr.workers[id] = nil // Placeholder
	wr.callbacks[id] = onMessageCallback
	wr.mu.Unlock()

	// Create a clean child context linked to the parent context
	workerCtx := NewContext(wr.parent.ctx)
	workerCtx.parent = wr.parent
	
	wr.mu.Lock()
	wr.workers[id] = workerCtx
	wr.mu.Unlock()

	// Increment parent WaitGroup synchronously to prevent the parent from exiting before startup
	workerCtx.WgAdd(1)

	// Start worker event loop in a background goroutine
	go func() {
		defer func() {
			workerCtx.Close()
			wr.mu.Lock()
			delete(wr.workers, id)
			delete(wr.callbacks, id)
			wr.mu.Unlock()
		}()

		content, err := os.ReadFile(scriptPath)
		if err != nil {
			fmt.Printf("Worker error reading script %s: %v\n", scriptPath, err)
			workerCtx.WgDone()
			return
		}

		// Inject worker-specific message ports
		wr.setupWorkerBindings(workerCtx, id)

		// Execute the script directly on the VM synchronously before entering event loop
		_, err = workerCtx.vm.RunScript(scriptPath, string(content))
		if err != nil {
			fmt.Printf("Worker script execution error %s: %v\n", scriptPath, err)
			workerCtx.setError(err)
			workerCtx.cancel()
			workerCtx.WgDone()
			return
		}
		
		// Startup complete, decrement the initial WaitGroup boost
		workerCtx.WgDone()
		
		_ = workerCtx.EventLoop()
	}()

	return id
}

// PostToWorker sends a message from parent context to the worker context.
func (wr *WorkerRegistry) PostToWorker(workerId int, msg interface{}) {
	wr.mu.Lock()
	workerCtx, ok := wr.workers[workerId]
	wr.mu.Unlock()

	if !ok || workerCtx == nil {
		return
	}

	workerCtx.Enqueue(func() {
		vm := workerCtx.VM()
		triggerVal := vm.Get("_triggerOnMessage")
		if trigger, ok := goja.AssertFunction(triggerVal); ok {
			_, _ = trigger(goja.Undefined(), vm.ToValue(msg))
		}
	})
}

// PostToMain sends a message from a worker context back to the parent context.
func (wr *WorkerRegistry) PostToMain(workerId int, msg interface{}) {
	wr.mu.Lock()
	callback, ok := wr.callbacks[workerId]
	wr.mu.Unlock()

	if !ok {
		return
	}

	wr.parent.Enqueue(func() {
		if callable, ok := goja.AssertFunction(callback); ok {
			// Wrap the message in a mock Event structure to match standard onmessage event syntax
			event := map[string]interface{}{
				"data": msg,
			}
			_, _ = callable(goja.Undefined(), wr.parent.VM().ToValue(event))
			// Flush parent microtasks after triggering the callback
			_, _ = wr.parent.VM().RunString("/* flush */")
		}
	})
}

// TerminateWorker closes a worker context prematurely.
func (wr *WorkerRegistry) TerminateWorker(workerId int) {
	wr.mu.Lock()
	workerCtx, ok := wr.workers[workerId]
	wr.mu.Unlock()

	if ok && workerCtx != nil {
		workerCtx.Close()
	}
}

func (wr *WorkerRegistry) setupWorkerBindings(workerCtx *Context, id int) {
	vm := workerCtx.VM()

	// Expose posting function back to main
	vm.Set("_goPostToMain", func(msg interface{}) {
		wr.PostToMain(id, msg)
	})

	setupScript := fmt.Sprintf(`
		(function(workerId) {
			globalThis.self = globalThis;
			globalThis.postMessage = function(msg) {
				_goPostToMain(msg);
			};
			globalThis._triggerOnMessage = function(msg) {
				if (typeof globalThis.onmessage === 'function') {
					globalThis.onmessage({ data: msg });
				}
			};
		})(%d);
	`, id)
	_, _ = vm.RunString(setupScript)
}

// SetupWorkers registers the global Worker class inside a parent context.
func SetupWorkers(parent *Context) {
	vm := parent.VM()
	registry := NewWorkerRegistry(parent)

	vm.Set("_goSpawnWorker", func(scriptPath string, onMessageCallback goja.Value) int {
		return registry.SpawnWorker(scriptPath, onMessageCallback)
	})

	vm.Set("_goPostToWorker", func(workerId int, msg interface{}) {
		registry.PostToWorker(workerId, msg)
	})

	vm.Set("_goTerminateWorker", func(workerId int) {
		registry.TerminateWorker(workerId)
	})

	workerShim := `
		class Worker {
			constructor(scriptPath) {
				this._id = _goSpawnWorker(scriptPath, (event) => {
					if (typeof this.onmessage === 'function') {
						this.onmessage(event);
					}
				});
			}

			postMessage(msg) {
				_goPostToWorker(this._id, msg);
			}

			terminate() {
				_goTerminateWorker(this._id);
			}
		}
		globalThis.Worker = Worker;
	`
	_, _ = vm.RunString(workerShim)
}
