package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

const (
	sessionCookie = "tunylo_session"
	unlockPath    = "/_tunylo/unlock"
	tokenBytes    = 32
	maxFailures   = 5
	failureWindow = 15 * time.Minute
	maxBodyBytes  = 4096
)

type attempt struct {
	count   int
	resetAt time.Time
}

type Proxy struct {
	passcode string
	token    string
	rp       *httputil.ReverseProxy
	srv      *http.Server
	mu       sync.Mutex
	attempts map[string]*attempt
}

func New(passcode, targetHost string, targetPort uint16) (*Proxy, error) {
	u, err := url.Parse(fmt.Sprintf("http://%s:%d", targetHost, targetPort))
	if err != nil {
		return nil, fmt.Errorf("invalid target: %w", err)
	}

	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generating session token: %w", err)
	}

	p := &Proxy{
		passcode: passcode,
		token:    hex.EncodeToString(raw),
		rp:       httputil.NewSingleHostReverseProxy(u),
		attempts: make(map[string]*attempt),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(unlockPath, p.handleUnlock)
	mux.HandleFunc("/", p.handleRequest)

	p.srv = &http.Server{
		Handler:      requestLogger(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return p, nil
}

func (p *Proxy) Start(ctx context.Context) (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("starting proxy: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	go func() {
		<-ctx.Done()
		p.srv.Close()
	}()
	go p.srv.Serve(ln) //nolint:errcheck
	return port, nil
}

func (p *Proxy) handleRequest(w http.ResponseWriter, r *http.Request) {
	if p.hasValidSession(r) {
		p.rp.ServeHTTP(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"passcode required"}`)) //nolint:errcheck
		return
	}
	p.serveUnlockPage(w, "")
}

func (p *Proxy) handleUnlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ip := clientIP(r)
	if p.isBlocked(ip) {
		http.Error(w, "too many failed attempts, try again later", http.StatusTooManyRequests)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	if !passcodeMatches(r.FormValue("passcode"), p.passcode) {
		p.recordFailure(ip)
		p.serveUnlockPage(w, "Incorrect passcode. Please try again.")
		return
	}
	p.resetFailures(ip)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    p.token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
