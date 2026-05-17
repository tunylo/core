package server

import (
	"fmt"
	"net/http"
	"time"
)

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// RequestEvent holds details about a single proxied request.
type RequestEvent struct {
	Method  string
	Path    string
	Status  int
	Bytes   int
	Latency time.Duration
	IP      string
}

func requestLogger(next http.Handler, onRequest func(RequestEvent)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if onRequest != nil {
			onRequest(RequestEvent{
				Method:  r.Method,
				Path:    r.URL.Path,
				Status:  rec.status,
				Bytes:   rec.bytes,
				Latency: time.Since(start),
				IP:      clientIP(r),
			})
		}
	})
}

func formatBytes(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fKB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%dB", n)
	}
}
