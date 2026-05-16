package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/fatih/color"
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

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		printRequest(r.Method, r.URL.Path, rec.status, rec.bytes, time.Since(start), clientIP(r))
	})
}

func printRequest(method, path string, status, bytes int, dur time.Duration, ip string) {
	statusColor := color.GreenString
	if status >= 400 && status < 500 {
		statusColor = color.YellowString
	} else if status >= 500 {
		statusColor = color.RedString
	}
	fmt.Printf("  %s %s %s %s %s %s\n",
		color.HiBlackString(ip),
		color.CyanString(method),
		path,
		statusColor(fmt.Sprintf("%d", status)),
		color.HiBlackString(formatBytes(bytes)),
		color.HiBlackString(dur.Round(time.Millisecond).String()),
	)
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
