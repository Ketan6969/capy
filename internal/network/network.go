package network

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"
	"time"

	"github.com/browserless/runtime/internal/engine"
	"github.com/dop251/goja"
)

// NetworkManager coordinates fetch and XMLHttpRequests for the runtime.
type NetworkManager struct {
	client        *http.Client
	Optimizer     *Optimizer
	totalRequests int
	mu            sync.Mutex
}

// NewNetworkManager creates a new NetworkManager.
func NewNetworkManager() *NetworkManager {
	jar, _ := cookiejar.New(nil)
	return &NetworkManager{
		client: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
		},
		Optimizer: NewOptimizer(),
	}
}

// RawResponse represents the serializable HTTP response structure exposed to Goja.
type RawResponse struct {
	Status     int               `json:"status"`
	StatusText string            `json:"statusText"`
	Body       string            `json:"body"`
	Headers    map[string]string `json:"headers"`
}

// SetupNetwork binds the fetch API into the engine context.
func (nm *NetworkManager) SetupNetwork(ctx *engine.Context) {
	nm.SetupNetworkInstrumented(ctx, nil)
}

// SetupNetworkInstrumented binds the fetch API and optionally increments counter on every call.
func (nm *NetworkManager) SetupNetworkInstrumented(ctx *engine.Context, counter *int) {
	vm := ctx.VM()

	// Expose low-level Go hook
	vm.Set("_goFetch", func(url string, options map[string]interface{}, callback goja.Value) {
		callable, ok := goja.AssertFunction(callback)
		if !ok {
			panic(vm.NewTypeError("_goFetch: callback must be a function"))
		}

		slog.Debug("Network request initiated", "url", url)

		// Increment network request counter if provided
		if counter != nil {
			*counter++
		}

		ctx.WgAdd(1)

		go func() {
			res, err := nm.performRequest(ctx.Ctx(), url, options)

			ctx.Enqueue(func() {
				defer ctx.WgDone()
				var errVal error
				if err != nil {
					_, errVal = callable(goja.Undefined(), vm.ToValue(err.Error()), goja.Undefined())
				} else {
					_, errVal = callable(goja.Undefined(), goja.Undefined(), vm.ToValue(res))
				}
				if errVal != nil {
					ctx.SetError(errVal)
					ctx.Cancel()
					return
				}

				// Flush Goja microtasks (like promise resolution callbacks) before WgDone runs
				_, errVal = vm.RunString("/* flush */")
				if errVal != nil {
					ctx.SetError(errVal)
					ctx.Cancel()
				}
			})
		}()
	})

	// Inject the ES6 standard fetch shim
	fetchScript := `
		(function() {
			class Response {
				constructor(rawRes) {
					this.status = rawRes.status;
					this.statusText = rawRes.statusText;
					this.ok = this.status >= 200 && this.status < 300;
					this._bodyText = rawRes.body;
					this.headers = new Map(Object.entries(rawRes.headers || {}));
				}

				async text() {
					return this._bodyText;
				}

				async json() {
					return JSON.parse(this._bodyText);
				}
			}

			globalThis.fetch = function(url, options) {
				return new Promise((resolve, reject) => {
					_goFetch(url, options || {}, (err, rawRes) => {
						if (err) {
							reject(new Error(err));
						} else {
							resolve(new Response(rawRes));
						}
					});
				});
			};
		})();
	`
	_, _ = vm.RunString(fetchScript)
}

func (nm *NetworkManager) GetTotalRequests() int {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	return nm.totalRequests
}

func (nm *NetworkManager) performRequest(ctx context.Context, urlStr string, options map[string]interface{}) (*RawResponse, error) {
	nm.mu.Lock()
	nm.totalRequests++
	nm.mu.Unlock()
	method := "GET"
	if m, ok := options["method"].(string); ok && m != "" {
		method = strings.ToUpper(m)
	}

	var bodyReader io.Reader
	if b, ok := options["body"].(string); ok && b != "" {
		bodyReader = strings.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return nil, err
	}

	// Add headers
	if headers, ok := options["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			if valStr, ok := v.(string); ok {
				req.Header.Set(k, valStr)
			}
		}
	}

	if nm.Optimizer.IsRecordMode() {
		reqHeaders := make(map[string]string)
		for k, v := range req.Header {
			if len(v) > 0 {
				reqHeaders[k] = v[0]
			}
		}
		var reqBody string
		if b, ok := options["body"].(string); ok {
			reqBody = b
		}
		nm.Optimizer.AddRecord(RequestRecord{
			URL:     urlStr,
			Method:  method,
			Headers: reqHeaders,
			Body:    reqBody,
		})
	}

	// Make the call
	resp, err := nm.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	resHeaders := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			resHeaders[k] = v[0]
		}
	}

	return &RawResponse{
		Status:     resp.StatusCode,
		StatusText: resp.Status,
		Body:       string(bodyBytes),
		Headers:    resHeaders,
	}, nil
}
