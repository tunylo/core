package tunnels

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/fatih/color"
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
	color.HiBlack("Downloading ngrok from %s", url)

	binDir, err := config.BinDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}

	outPath := t.BinaryPath()
	if err := installFromURL(url, true, outPath); err != nil {
		return fmt.Errorf("failed to install ngrok: %w", err)
	}
	if err := os.Chmod(outPath, 0o755); err != nil {
		return err
	}

	color.Green("ngrok installed successfully.")
	return nil
}

func (t *NgrokTunnel) Configure(cfg *config.Config) error {
	color.New(color.Bold).Println("ngrok configuration")
	token := ""
	if cfg.TunnelTokens != nil {
		token = cfg.TunnelTokens["ngrok"]
	}
	if token == "" {
		color.HiBlack("No auth token provided. Sign up free at https://ngrok.com")
		color.HiBlack("Run: tunylo configure --token <YOUR_TOKEN> to save it.")
		return nil
	}
	binary, err := exec.LookPath(t.BinaryPath())
	if err != nil {
		binary, err = exec.LookPath(t.BinaryName())
		if err != nil {
			color.HiBlack("ngrok binary not found; token saved, will be applied on first run.")
			return nil
		}
	}
	cmd := exec.Command(binary, "config", "add-authtoken", token)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to apply ngrok auth token: %w", err)
	}
	color.Green("ngrok auth token applied.")
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
	if cfg != nil && cfg.TunnelTokens != nil {
		if token := cfg.TunnelTokens["ngrok"]; token != "" {
			args = append(args, "--authtoken", token)
		}
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
