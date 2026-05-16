package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTunnelAlreadyRunning_Live(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")
	os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0o644)

	pid, running := TunnelAlreadyRunning(pidPath)
	if !running {
		t.Fatal("expected live process to be detected as running")
	}
	if pid != os.Getpid() {
		t.Fatalf("expected PID %d, got %d", os.Getpid(), pid)
	}
}

func TestTunnelAlreadyRunning_Stale(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")
	os.WriteFile(pidPath, []byte("99999999"), 0o644)

	_, running := TunnelAlreadyRunning(pidPath)
	if running {
		t.Fatal("stale PID should not be reported as running")
	}
}

func TestTunnelAlreadyRunning_Missing(t *testing.T) {
	_, running := TunnelAlreadyRunning(filepath.Join(t.TempDir(), "absent.pid"))
	if running {
		t.Fatal("missing file should not be reported as running")
	}
}

func TestTunnelAlreadyRunning_InvalidContent(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")
	os.WriteFile(pidPath, []byte("notanumber"), 0o644)

	_, running := TunnelAlreadyRunning(pidPath)
	if running {
		t.Fatal("invalid content should not be reported as running")
	}
}

func TestWaitForURL_Found(t *testing.T) {
	f, _ := os.CreateTemp(t.TempDir(), "*.log")
	f.WriteString("connecting to https://example.trycloudflare.com\n")
	f.Close()

	url, ok := WaitForURL(f.Name(), 2*time.Second)
	if !ok {
		t.Fatal("expected URL to be found")
	}
	if url != "https://example.trycloudflare.com" {
		t.Fatalf("unexpected URL: %s", url)
	}
}

func TestWaitForURL_Timeout(t *testing.T) {
	f, _ := os.CreateTemp(t.TempDir(), "*.log")
	f.WriteString("no url here\n")
	f.Close()

	_, ok := WaitForURL(f.Name(), 300*time.Millisecond)
	if ok {
		t.Fatal("expected timeout, not a URL")
	}
}

func TestWaitForURL_AnsiStripped(t *testing.T) {
	f, _ := os.CreateTemp(t.TempDir(), "*.log")
	f.WriteString("\x1b[32mhttps://ansi.trycloudflare.com\x1b[0m\n")
	f.Close()

	url, ok := WaitForURL(f.Name(), 2*time.Second)
	if !ok {
		t.Fatal("expected URL to be found after stripping ANSI")
	}
	if url != "https://ansi.trycloudflare.com" {
		t.Fatalf("unexpected URL: %s", url)
	}
}
