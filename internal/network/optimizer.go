package network

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// RequestRecord stores serializable HTTP request details.
type RequestRecord struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

// ResponseRecord stores serializable HTTP response details.
type ResponseRecord struct {
	URL        string            `json:"url"`
	Status     int               `json:"status"`
	StatusText string            `json:"statusText"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}

// RulesFile defines the JSON structure for the record file.
type RulesFile struct {
	Requests []RequestRecord `json:"requests"`
}

// ReplayResult stores the consolidated results of a replay execution.
type ReplayResult struct {
	Results []ResponseRecord `json:"results"`
}

// Optimizer coordinates recording and replaying of API fetches.
type Optimizer struct {
	mu         sync.Mutex
	recordMode bool
	records    []RequestRecord
	client     *http.Client
}

// NewOptimizer creates a new Optimizer.
func NewOptimizer() *Optimizer {
	return &Optimizer{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetRecordMode enables or disables record mode.
func (opt *Optimizer) SetRecordMode(active bool) {
	opt.mu.Lock()
	defer opt.mu.Unlock()
	opt.recordMode = active
}

// IsRecordMode returns whether recording is active.
func (opt *Optimizer) IsRecordMode() bool {
	opt.mu.Lock()
	defer opt.mu.Unlock()
	return opt.recordMode
}

// AddRecord adds a request to the record set if record mode is enabled.
func (opt *Optimizer) AddRecord(rec RequestRecord) {
	opt.mu.Lock()
	defer opt.mu.Unlock()
	if opt.recordMode {
		opt.records = append(opt.records, rec)
	}
}

// Save writes all recorded requests to the specified path.
func (opt *Optimizer) Save(rulesPath string) error {
	opt.mu.Lock()
	defer opt.mu.Unlock()

	rules := RulesFile{
		Requests: opt.records,
	}

	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(rulesPath, data, 0644)
}

// Replay loads a recorded request list, fetches them all directly, and returns a JSON payload.
func (opt *Optimizer) Replay(rulesPath string) ([]byte, error) {
	data, err := os.ReadFile(rulesPath)
	if err != nil {
		return nil, err
	}

	var rules RulesFile
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, err
	}

	var results []ResponseRecord

	for _, reqRec := range rules.Requests {
		var bodyReader io.Reader
		if reqRec.Body != "" {
			bodyReader = strings.NewReader(reqRec.Body)
		}
		req, err := http.NewRequest(reqRec.Method, reqRec.URL, bodyReader)
		if err != nil {
			return nil, err
		}

		for k, v := range reqRec.Headers {
			req.Header.Set(k, v)
		}

		resp, err := opt.client.Do(req)
		if err != nil {
			results = append(results, ResponseRecord{
				URL:    reqRec.URL,
				Status: 0,
				Body:   err.Error(),
			})
			continue
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			results = append(results, ResponseRecord{
				URL:    reqRec.URL,
				Status: resp.StatusCode,
				Body:   err.Error(),
			})
			continue
		}

		resHeaders := make(map[string]string)
		for k, v := range resp.Header {
			if len(v) > 0 {
				resHeaders[k] = v[0]
			}
		}

		results = append(results, ResponseRecord{
			URL:        reqRec.URL,
			Status:     resp.StatusCode,
			StatusText: resp.Status,
			Headers:    resHeaders,
			Body:       string(bodyBytes),
		})
	}

	replayRes := ReplayResult{
		Results: results,
	}

	return json.MarshalIndent(replayRes, "", "  ")
}
