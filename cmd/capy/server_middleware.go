package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ctxKey string

const requestIDContextKey ctxKey = "request_id"

type serverConfig struct {
	APIKey             string
	RateLimitPerMinute int
}

func loadServerConfig() serverConfig {
	rateLimit := 60
	if raw := strings.TrimSpace(getenv("BROWSERLESS_RATE_LIMIT_PER_MINUTE", "")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			rateLimit = parsed
		}
	}

	return serverConfig{
		APIKey:             strings.TrimSpace(getenv("BROWSERLESS_API_KEY", "")),
		RateLimitPerMinute: rateLimit,
	}
}

type rateLimiter struct {
	mu      sync.Mutex
	clients map[string]*rateLimitEntry
}

type rateLimitEntry struct {
	windowStart time.Time
	count       int
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		clients: make(map[string]*rateLimitEntry),
	}
}

func (rl *rateLimiter) allow(key string, now time.Time, limit int) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, ok := rl.clients[key]
	if !ok || now.Sub(entry.windowStart) >= time.Minute {
		rl.clients[key] = &rateLimitEntry{
			windowStart: now,
			count:       1,
		}
		return true
	}

	if entry.count >= limit {
		return false
	}

	entry.count++
	return true
}

type serverApp struct {
	cfg        serverConfig
	rateLimits *rateLimiter
}

func newServerHandler(cfg serverConfig) http.Handler {
	app := &serverApp{
		cfg:        cfg,
		rateLimits: newRateLimiter(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", app.handleHealth)
	mux.HandleFunc("/readyz", app.handleHealth)
	mux.HandleFunc("/extract", app.handleExtract)

	return app.withMiddleware(mux)
}

func (app *serverApp) withMiddleware(next http.Handler) http.Handler {
	return app.loggingMiddleware(app.requestIDMiddleware(app.authMiddleware(app.rateLimitMiddleware(next))))
}

func (app *serverApp) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if requestID == "" {
			requestID = newRequestID()
		}
		w.Header().Set("X-Request-Id", requestID)
		ctx := context.WithValue(r.Context(), requestIDContextKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (app *serverApp) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(recorder, r)

		slog.Info("http_request",
			"request_id", requestIDFromContext(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", clientIP(r),
			"status", recorder.statusCode,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func (app *serverApp) authMiddleware(next http.Handler) http.Handler {
	requiresAuth := app.cfg.APIKey != ""

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requiresAuth || r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}

		apiKey := strings.TrimSpace(r.Header.Get("X-API-Key"))
		if apiKey == "" {
			authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			if strings.HasPrefix(authHeader, "Bearer ") {
				apiKey = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			}
		}

		if apiKey != app.cfg.APIKey {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (app *serverApp) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}

		if app.cfg.RateLimitPerMinute <= 0 {
			next.ServeHTTP(w, r)
			return
		}

		if !app.rateLimits.allow(clientIP(r), time.Now().UTC(), app.cfg.RateLimitPerMinute) {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (app *serverApp) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (sr *statusRecorder) WriteHeader(statusCode int) {
	sr.statusCode = statusCode
	sr.ResponseWriter.WriteHeader(statusCode)
}

func requestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDContextKey).(string)
	return value
}

func newRequestID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(buf)
}

func clientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
