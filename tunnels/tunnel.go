package tunnels

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/tunylo/core/config"
)

type TunnelProcess struct {
	PublicURL *string
	Cmd       *exec.Cmd
}

type Tunnel interface {
	Label() string
	Key() string
	BinaryName() string
	BinaryPath() string
	IsInstalled() bool
	Install() error
	Configure(cfg *config.Config) error
	Start(host string, port uint16) (*TunnelProcess, error)
	Uninstall() error
}

func DefaultBinaryPath(binaryName string) string {
	dir, err := config.BinDir()
	if err != nil {
		return binaryName
	}
	return filepath.Join(dir, binaryName)
}

func DefaultIsInstalled(binaryPath string) bool {
	if _, err := exec.LookPath(binaryPath); err == nil {
		return true
	}
	_, err := exec.LookPath(filepath.Base(binaryPath))
	return err == nil
}

func newCmd(binary string, args ...string) *exec.Cmd {
	return exec.Command(binary, args...)
}

func defaultUninstall(binaryPath string) error {
	if err := os.Remove(binaryPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func AllTunnels() []Tunnel {
	return []Tunnel{
		&CloudflareTunnel{},
	}
}
