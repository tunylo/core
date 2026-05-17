package tunnels

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/tunylo/core/config"
)

type CloudflareTunnel struct{}

func (t *CloudflareTunnel) Label() string { return "Cloudflare Tunnel" }
func (t *CloudflareTunnel) Key() string   { return "cloudflare" }

func (t *CloudflareTunnel) BinaryName() string {
	if runtime.GOOS == "windows" {
		return "cloudflared.exe"
	}
	return "cloudflared"
}

func (t *CloudflareTunnel) BinaryPath() string {
	return DefaultBinaryPath(t.BinaryName())
}

func (t *CloudflareTunnel) IsInstalled() bool {
	return DefaultIsInstalled(t.BinaryPath())
}

func (t *CloudflareTunnel) downloadURL() (string, bool) {
	goos := runtime.GOOS
	arch := runtime.GOARCH
	if arch != "arm64" {
		arch = "amd64"
	}
	switch goos {
	case "darwin":
		return fmt.Sprintf(
			"https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-darwin-%s.tgz",
			arch,
		), true
	case "windows":
		return fmt.Sprintf(
			"https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-%s.exe",
			arch,
		), false
	default:
		return fmt.Sprintf(
			"https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-%s",
			arch,
		), false
	}
}

func (t *CloudflareTunnel) Install() error {
	url, isTgz := t.downloadURL()

	binDir, err := config.BinDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}

	outPath := t.BinaryPath()
	if err := installFromURL(url, isTgz, outPath); err != nil {
		return err
	}

	if err := os.Chmod(outPath, 0o755); err != nil {
		return err
	}

	return nil
}

func (t *CloudflareTunnel) ConfigFields() []ConfigField { return nil }

func installFromURL(url string, isTgz bool, outPath string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "tunylo/0.1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s downloading binary", resp.Status)
	}

	if isTgz {
		if err := extractTgz(resp.Body, "cloudflared", outPath); err != nil {
			return fmt.Errorf("failed to extract binary: %w", err)
		}
		return nil
	}

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("failed to write binary: %w", err)
	}
	return nil
}

func (t *CloudflareTunnel) Configure(cfg *config.Config) error {
	return nil
}

func (t *CloudflareTunnel) Start(host string, port uint16) (*TunnelProcess, error) {
	binary, err := exec.LookPath(t.BinaryPath())
	if err != nil {
		binary, err = exec.LookPath(t.BinaryName())
		if err != nil {
			return nil, fmt.Errorf("cloudflared binary not found")
		}
	}

	url := fmt.Sprintf("http://%s:%d", host, port)

	cmd := newCmd(binary, "tunnel", "--url", url)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start cloudflared: %w", err)
	}

	urlCh := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			for _, field := range strings.Fields(line) {
				if strings.HasPrefix(field, "https://") && strings.Contains(field, "trycloudflare.com") {
					urlCh <- field
					return
				}
			}
		}
		close(urlCh)
	}()

	var publicURL *string
	if u, ok := <-urlCh; ok {
		publicURL = &u
	}

	return &TunnelProcess{PublicURL: publicURL, Cmd: cmd}, nil
}

func (t *CloudflareTunnel) Uninstall() error {
	return defaultUninstall(t.BinaryPath())
}

func extractTgz(r io.Reader, targetName string, destPath string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Name == targetName || strings.HasSuffix(hdr.Name, "/"+targetName) {
			f, err := os.Create(destPath)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("binary %q not found in archive", targetName)
}
