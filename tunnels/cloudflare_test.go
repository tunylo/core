package tunnels

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var cloudflareTunnel = &CloudflareTunnel{}

func TestCloudflareTunnel_Metadata(t *testing.T) {
	if cloudflareTunnel.Label() != "Cloudflare Tunnel" {
		t.Errorf("unexpected label: %s", cloudflareTunnel.Label())
	}
	if cloudflareTunnel.Key() != "cloudflare" {
		t.Errorf("unexpected key: %s", cloudflareTunnel.Key())
	}
}

func TestCloudflareTunnel_BinaryName(t *testing.T) {
	name := cloudflareTunnel.BinaryName()
	if runtime.GOOS == "windows" {
		if name != "cloudflared.exe" {
			t.Errorf("expected cloudflared.exe on windows, got %s", name)
		}
	} else {
		if name != "cloudflared" {
			t.Errorf("expected cloudflared, got %s", name)
		}
	}
}

func TestCloudflareTunnel_DownloadURL(t *testing.T) {
	url, _ := cloudflareTunnel.downloadURL()
	if !strings.Contains(url, "cloudflared") {
		t.Errorf("expected cloudflared in URL, got %s", url)
	}
	if !strings.HasPrefix(url, "https://") {
		t.Errorf("expected https URL, got %s", url)
	}
	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(url, "darwin") || !strings.HasSuffix(url, ".tgz") {
			t.Errorf("expected darwin .tgz URL, got %s", url)
		}
	case "windows":
		if !strings.Contains(url, "windows") || !strings.HasSuffix(url, ".exe") {
			t.Errorf("expected windows .exe URL, got %s", url)
		}
	default:
		if !strings.Contains(url, "linux") {
			t.Errorf("expected linux URL, got %s", url)
		}
	}
}

func TestCloudflareTunnel_IsInstalled_NotInstalled(t *testing.T) {
	dir := t.TempDir()
	fakePath := filepath.Join(dir, "cloudflared-notexist")
	if DefaultIsInstalled(fakePath) {
		t.Fatal("expected not installed for nonexistent binary")
	}
}

func TestCloudflareTunnel_IsInstalled_Installed(t *testing.T) {
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "cloudflared")
	os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755)
	if !DefaultIsInstalled(fakeBin) {
		t.Fatal("expected installed for existing binary")
	}
}

func TestCloudflareTunnel_Uninstall(t *testing.T) {
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "cloudflared")
	os.WriteFile(fakeBin, []byte("binary"), 0o755)

	if err := defaultUninstall(fakeBin); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if _, err := os.Stat(fakeBin); !os.IsNotExist(err) {
		t.Fatal("expected binary to be removed")
	}
}

func TestCloudflareTunnel_Uninstall_NotExist(t *testing.T) {
	if err := defaultUninstall("/nonexistent/path/cloudflared"); err != nil {
		t.Fatalf("uninstall of nonexistent binary should not error: %v", err)
	}
}

func TestExtractTgz(t *testing.T) {
	content := []byte("fake cloudflared binary")
	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: "cloudflared", Size: int64(len(content)), Mode: 0o755})
	tw.Write(content)
	tw.Close()
	gz.Close()

	dest := filepath.Join(t.TempDir(), "cloudflared")
	if err := extractTgz(buf, "cloudflared", dest); err != nil {
		t.Fatalf("extractTgz: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("extracted content mismatch: got %q", got)
	}
}

func TestExtractTgz_MissingBinary(t *testing.T) {
	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: "other-file", Size: 4, Mode: 0o644})
	tw.Write([]byte("data"))
	tw.Close()
	gz.Close()

	dest := filepath.Join(t.TempDir(), "cloudflared")
	err := extractTgz(buf, "cloudflared", dest)
	if err == nil {
		t.Fatal("expected error when binary not found in archive")
	}
}

func TestCloudflareTunnel_Install_BadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "cloudflared")

	err := installFromURL(srv.URL, false, "cloudflared", fakeBin)
	if err == nil {
		t.Fatal("expected error on non-200 response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error, got: %v", err)
	}
}

func TestCloudflareTunnel_Install_PlainBinary(t *testing.T) {
	content := []byte("fake binary content")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))
	defer srv.Close()

	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "cloudflared")

	if err := installFromURL(srv.URL, false, "cloudflared", fakeBin); err != nil {
		t.Fatalf("install: %v", err)
	}
	got, _ := os.ReadFile(fakeBin)
	if !bytes.Equal(got, content) {
		t.Errorf("binary content mismatch")
	}
}

func TestAllTunnels(t *testing.T) {
	all := AllTunnels()
	if len(all) == 0 {
		t.Fatal("expected at least one tunnel")
	}
	keys := map[string]bool{}
	for _, tun := range all {
		if tun.Key() == "" {
			t.Error("tunnel has empty key")
		}
		if tun.Label() == "" {
			t.Error("tunnel has empty label")
		}
		if keys[tun.Key()] {
			t.Errorf("duplicate tunnel key: %s", tun.Key())
		}
		keys[tun.Key()] = true
	}
}
