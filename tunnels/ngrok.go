package tunnels

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/tunylo/core/config"
)

type NgrokTunnel struct{}

func (t *NgrokTunnel) Label() string { return "ngrok" }
func (t *NgrokTunnel) Key() string   { return "ngrok" }

func (t *NgrokTunnel) BinaryName() string {
	if runtime.GOOS == "windows" {
		return "ngrok.exe"
	}
	return "ngrok"
}

func (t *NgrokTunnel) BinaryPath() string {
	return DefaultBinaryPath(t.BinaryName())
}

func (t *NgrokTunnel) IsInstalled() bool {
	return DefaultIsInstalled(t.BinaryPath())
}

func (t *NgrokTunnel) downloadURL() string {
	goos := runtime.GOOS
	arch := runtime.GOARCH
	if arch != "arm64" {
		arch = "amd64"
	}
	switch goos {
	case "darwin":
		return fmt.Sprintf("https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable-darwin-%s.tgz", arch)
	case "windows":
		return fmt.Sprintf("https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable-windows-%s.zip", arch)
	default:
		return fmt.Sprintf("https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable-linux-%s.tgz", arch)
	}
}

func (t *NgrokTunnel) Install() error {
	url := t.downloadURL()

	binDir, err := config.BinDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}

	outPath := t.BinaryPath()
	if err := installFromURL(url, true, t.BinaryName(), outPath); err != nil {
		return fmt.Errorf("failed to install ngrok: %w", err)
	}
	if err := os.Chmod(outPath, 0o755); err != nil {
		return err
	}
	return nil
}

func (t *NgrokTunnel) ConfigFields() []ConfigField {
	return []ConfigField{
		{Key: "authtoken", Label: "Auth token", Secret: true, Required: false},
	}
}

func (t *NgrokTunnel) Configure(cfg *config.Config) error {
	token := cfg.TunnelValue("ngrok", "authtoken")
	if token == "" {
		return nil
	}
	binary, err := exec.LookPath(t.BinaryPath())
	if err != nil {
		binary, err = exec.LookPath(t.BinaryName())
		if err != nil {
			// Binary not installed yet; token is saved and will be applied on first run.
			return nil
		}
	}
	cmd := exec.Command(binary, "config", "add-authtoken", token)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to apply ngrok auth token: %w", err)
	}
	return nil
}

func (t *NgrokTunnel) Start(host string, port uint16) (*TunnelProcess, error) {
	binary, err := exec.LookPath(t.BinaryPath())
	if err != nil {
		binary, err = exec.LookPath(t.BinaryName())
		if err != nil {
			return nil, fmt.Errorf("ngrok binary not found")
		}
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	args := []string{"http", addr}

	cfg, _ := config.Load()
	if token := cfg.TunnelValue("ngrok", "authtoken"); token != "" {
		args = append(args, "--authtoken", token)
	}

	cmd := newCmd(binary, args...)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ngrok: %w", err)
	}

	publicURL := pollNgrokURL(15 * time.Second)

	return &TunnelProcess{PublicURL: publicURL, Cmd: cmd}, nil
}

func (t *NgrokTunnel) Uninstall() error {
	return defaultUninstall(t.BinaryPath())
}

// pollNgrokURL polls the ngrok local API until a tunnel URL is available.
func pollNgrokURL(timeout time.Duration) *string {
	type tunnel struct {
		PublicURL string `json:"public_url"`
		Proto     string `json:"proto"`
	}
	type response struct {
		Tunnels []tunnel `json:"tunnels"`
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://localhost:4040/api/tunnels") //nolint:noctx
		if err == nil {
			var r response
			if json.NewDecoder(resp.Body).Decode(&r) == nil {
				resp.Body.Close()
				for _, tun := range r.Tunnels {
					if tun.Proto == "https" && tun.PublicURL != "" {
						u := tun.PublicURL
						return &u
					}
				}
			} else {
				resp.Body.Close()
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}
