package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestProxy(t *testing.T, passcode string) *Proxy {
	t.Helper()
	p, err := New(passcode, "127.0.0.1", 9)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

// --- passcode ---

func TestPasscodeMatches(t *testing.T) {
	cases := []struct {
		got, want string
		match     bool
	}{
		{"secret", "secret", true},
		{"wrong", "secret", false},
		{"", "secret", false},
		{"secret", "", false},
		{"", "", true},
	}
	for _, c := range cases {
		if passcodeMatches(c.got, c.want) != c.match {
			t.Errorf("passcodeMatches(%q, %q) = %v, want %v", c.got, c.want, !c.match, c.match)
		}
	}
}

func TestHasValidSession(t *testing.T) {
	p := newTestProxy(t, "abc")

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if p.hasValidSession(r) {
		t.Fatal("expected no session without cookie")
	}

	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: p.token})
	if !p.hasValidSession(r) {
		t.Fatal("expected valid session with correct token")
	}

	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2.AddCookie(&http.Cookie{Name: sessionCookie, Value: "bad"})
	if p.hasValidSession(r2) {
		t.Fatal("expected invalid session with wrong token")
	}
}

func TestRateLimiting(t *testing.T) {
	p := newTestProxy(t, "abc")
	ip := "1.2.3.4"

	for i := 0; i < maxFailures; i++ {
		if p.isBlocked(ip) {
			t.Fatalf("should not be blocked after %d failures", i)
		}
		p.recordFailure(ip)
	}
	if !p.isBlocked(ip) {
		t.Fatal("expected blocked after max failures")
	}
	p.resetFailures(ip)
	if p.isBlocked(ip) {
		t.Fatal("expected unblocked after reset")
	}
}

func TestHandleUnlock_WrongPasscode(t *testing.T) {
	p := newTestProxy(t, "correct")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, unlockPath, strings.NewReader("passcode=wrong"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	p.handleUnlock(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (unlock page), got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Incorrect passcode") {
		t.Fatal("expected error message in response")
	}
}

func TestHandleUnlock_CorrectPasscode(t *testing.T) {
	p := newTestProxy(t, "correct")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, unlockPath, strings.NewReader("passcode=correct"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	p.handleUnlock(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", w.Code)
	}
	var found bool
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookie && c.Value == p.token {
			found = true
		}
	}
	if !found {
		t.Fatal("expected session cookie to be set")
	}
}

func TestHandleUnlock_MethodNotAllowed(t *testing.T) {
	p := newTestProxy(t, "abc")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, unlockPath, nil)
	p.handleUnlock(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleUnlock_Blocked(t *testing.T) {
	p := newTestProxy(t, "abc")
	ip := "5.6.7.8"
	for i := 0; i < maxFailures; i++ {
		p.recordFailure(ip)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, unlockPath, strings.NewReader("passcode=abc"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.RemoteAddr = ip + ":1234"

	p.handleUnlock(w, r)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
}

func TestHandleUnlock_EmptyPasscode(t *testing.T) {
	p := newTestProxy(t, "correct")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, unlockPath, strings.NewReader("passcode="))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	p.handleUnlock(w, r)
	if w.Code == http.StatusSeeOther {
		t.Fatal("empty passcode should not unlock")
	}
}

// --- request handler ---

func TestHandleRequest_NoSession(t *testing.T) {
	p := newTestProxy(t, "abc")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	p.handleRequest(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (unlock page), got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Access Required") {
		t.Fatal("expected unlock page")
	}
}

func TestHandleRequest_NonGetNoSession(t *testing.T) {
	p := newTestProxy(t, "abc")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/data", nil)
	p.handleRequest(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleRequest_WithValidSession(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello from backend"))
	}))
	defer backend.Close()

	var backendPort uint16
	fmt.Sscanf(backend.URL[len("http://127.0.0.1:"):], "%d", &backendPort)

	p, err := New("secret", "127.0.0.1", backendPort)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: p.token})
	p.handleRequest(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from backend, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "hello from backend") {
		t.Fatalf("expected backend response, got: %s", w.Body.String())
	}
}

// --- request logging ---

func TestResponseRecorder_Status(t *testing.T) {
	inner := httptest.NewRecorder()
	rec := &responseRecorder{ResponseWriter: inner, status: http.StatusOK}
	rec.WriteHeader(http.StatusCreated)
	if rec.status != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.status)
	}
}

func TestResponseRecorder_Bytes(t *testing.T) {
	inner := httptest.NewRecorder()
	rec := &responseRecorder{ResponseWriter: inner, status: http.StatusOK}
	n, err := rec.Write([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if rec.bytes != n {
		t.Fatalf("expected bytes %d, got %d", n, rec.bytes)
	}
}

func TestResponseRecorder_MultiWrite(t *testing.T) {
	inner := httptest.NewRecorder()
	rec := &responseRecorder{ResponseWriter: inner, status: http.StatusOK}
	rec.Write([]byte("foo"))
	rec.Write([]byte("bar"))
	if rec.bytes != 6 {
		t.Fatalf("expected 6 bytes total, got %d", rec.bytes)
	}
}

func TestRequestLogger_CapturesStatus(t *testing.T) {
	handler := requestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("teapot"))
	}), nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/brew", nil)
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusTeapot {
		t.Fatalf("expected 418, got %d", w.Code)
	}
}

func TestRequestLogger_PassesBodyThrough(t *testing.T) {
	body := []byte("response body")
	handler := requestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}), nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)
	got, _ := io.ReadAll(w.Body)
	if !bytes.Equal(got, body) {
		t.Fatalf("body mismatch: got %q", got)
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, "0B"},
		{512, "512B"},
		{1024, "1.0KB"},
		{1536, "1.5KB"},
		{1 << 20, "1.0MB"},
	}
	for _, c := range cases {
		if got := formatBytes(c.n); got != c.want {
			t.Errorf("formatBytes(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

// --- proxy lifecycle ---

func TestProxyStart(t *testing.T) {
	p := newTestProxy(t, "abc")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port, err := p.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if port <= 0 {
		t.Fatalf("expected valid port, got %d", port)
	}
}

func TestClientIP_RemoteAddr(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:5000"
	if ip := clientIP(r); ip != "10.0.0.1" {
		t.Errorf("expected 10.0.0.1, got %s", ip)
	}
}

func TestClientIP_CFHeader(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "127.0.0.1:5000"
	r.Header.Set("CF-Connecting-IP", "203.0.113.5")
	if ip := clientIP(r); ip != "203.0.113.5" {
		t.Errorf("expected CF IP 203.0.113.5, got %s", ip)
	}
}
