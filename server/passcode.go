package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"html"
	"net"
	"net/http"
	"strings"
	"time"
)

func (p *Proxy) hasValidSession(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	got := sha256.Sum256([]byte(c.Value))
	want := sha256.Sum256([]byte(p.token))
	return subtle.ConstantTimeCompare(got[:], want[:]) == 1
}

func passcodeMatches(got, want string) bool {
	gotH := sha256.Sum256([]byte(got))
	wantH := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(gotH[:], wantH[:]) == 1
}

func (p *Proxy) isBlocked(ip string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	a, ok := p.attempts[ip]
	if !ok {
		return false
	}
	if time.Now().After(a.resetAt) {
		delete(p.attempts, ip)
		return false
	}
	return a.count >= maxFailures
}

func (p *Proxy) recordFailure(ip string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	a, ok := p.attempts[ip]
	if !ok || time.Now().After(a.resetAt) {
		p.attempts[ip] = &attempt{count: 1, resetAt: time.Now().Add(failureWindow)}
		return
	}
	a.count++
}

func (p *Proxy) resetFailures(ip string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.attempts, ip)
}

func clientIP(r *http.Request) string {
	if cfIP := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cfIP != "" {
		if net.ParseIP(cfIP) != nil {
			return cfIP
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (p *Proxy) serveUnlockPage(w http.ResponseWriter, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	errHTML := ""
	if errMsg != "" {
		errHTML = `<p class="error">` + html.EscapeString(errMsg) + `</p>`
	}
	fmt.Fprintf(w, unlockPageHTML, errHTML) //nolint:errcheck
}

const unlockPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>tunylo — Access Required</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: #0f0f0f; color: #e2e2e2;
      display: flex; align-items: center; justify-content: center;
      min-height: 100vh;
    }
    .card {
      background: #1a1a1a; border: 1px solid #2e2e2e; border-radius: 12px;
      padding: 2.5rem 2rem; width: 100%%; max-width: 360px; text-align: center;
    }
    .lock { font-size: 2rem; margin-bottom: 1rem; }
    h1 { font-size: 1.1rem; font-weight: 600; margin-bottom: 0.4rem; }
    .sub { font-size: 0.85rem; color: #888; margin-bottom: 1.5rem; }
    .error { color: #f87171; font-size: 0.85rem; margin-bottom: 0.9rem; }
    input[type=password] {
      width: 100%%; padding: 0.65rem 0.9rem; border-radius: 8px;
      border: 1px solid #333; background: #111; color: #e2e2e2;
      font-size: 0.95rem; outline: none; margin-bottom: 0.9rem;
    }
    input[type=password]:focus { border-color: #555; }
    button {
      width: 100%%; padding: 0.65rem; border-radius: 8px;
      background: #e2e2e2; color: #111; font-weight: 600;
      font-size: 0.95rem; border: none; cursor: pointer;
    }
    button:hover { background: #fff; }
  </style>
</head>
<body>
  <div class="card">
    <div class="lock">🔒</div>
    <h1>Access Required</h1>
    <p class="sub">This tunnel is passcode-protected. Enter the passcode to continue.</p>
    %s
    <form method="POST" action="/_tunylo/unlock">
      <input type="password" name="passcode" placeholder="Enter passcode" autofocus required>
      <button type="submit">Unlock</button>
    </form>
  </div>
</body>
</html>
`
