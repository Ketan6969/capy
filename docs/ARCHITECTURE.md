# Browserless Runtime Architecture

The `browserless` project is an experimental lightweight headless browser runtime written in Go. Instead of running a full Chromium instance via Playwright/Puppeteer, `browserless` uses the [Goja](https://github.com/dop251/goja) ECMAScript engine to evaluate JavaScript natively within Go, backed by a custom set of polyfills that mock browser APIs.

## Core Components

The runtime is separated into three main isolated subsystems:

### 1. `internal/engine`
The **Engine** manages execution contexts. 
- **Context Management**: Wraps the raw `goja.Runtime` instance and provides an isolated execution environment.
- **Event Loop**: Goja does not have a built-in event loop for `setTimeout` or Promises. The `engine.Context` implements a custom single-threaded job queue. Any asynchronous JS task is queued on the Go channel `jobs` and executed sequentially until the queue drains.
- **Web Workers**: Supports spinning up background JavaScript threads (`Worker` API). Each worker is given its own `goja.Runtime` and isolated `Context`, executing concurrently in a separate Go goroutine. Communication between the main thread and workers is bridged using Go channels (`PostToWorker` and `PostToMain`).

### 2. `internal/dom`
The **DOM** package maps Go structures to JavaScript objects.
- **`dom.Node`**: The unified data structure representing HTML elements. Rather than defining distinct Go types for `HTMLDivElement` and `HTMLSpanElement`, everything is represented by `*dom.Node`.
- **Property Binding**: Uses Go reflection to map Go struct fields (e.g., `Node.ChildNodes`, `Node.ClassName`) into JavaScript properties. 
- **JS Polyfills**: The native `internal/polyfills/dom.js` script injects standard web APIs (`document.createElement`, `document.getElementById`) which internally call into the native Go structures.

### 3. `internal/network`
The **NetworkManager** mocks asynchronous network requests.
- **`fetch` & `XMLHttpRequest`**: Bound globally into the `goja.Runtime`. When a JS script calls `fetch`, the call is intercepted, delegated to Go's native `http.Client`, and then resolved asynchronously back onto the engine's event loop via `runCtx.Enqueue()`.
- **Cookie Management**: Leverages Go's `http.CookieJar` to persist session state across fetches and document loads.

## Memory & Concurrency Model

- **Single Threaded**: The Goja VM is strictly single-threaded. You cannot share a single `Context` across multiple goroutines.
- **No Shared State**: Each executed script is bound to a transient `Context` which is destroyed upon completion, preventing memory leaks between runs.
