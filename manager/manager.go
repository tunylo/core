package manager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tunylo/core/config"
	"github.com/tunylo/core/daemon"
)

type Session struct {
	Name string
	PID  int
	Live bool
}

func StopNamed(name string) (int, error) {
	pidPath, err := config.NamedPidPath(name)
	if err != nil {
		return 0, err
	}

	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, fmt.Errorf("no background tunnel named %q found", name)
	}

	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil || pid <= 0 {
		return 0, fmt.Errorf("invalid PID file for %q", name)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return 0, fmt.Errorf("could not find process %d: %w", pid, err)
	}

	if err := proc.Signal(os.Interrupt); err != nil {
		return 0, fmt.Errorf("could not stop process %d: %w", pid, err)
	}

	os.Remove(pidPath) //nolint:errcheck
	return pid, nil
}

func ListRunning() ([]Session, error) {
	base, err := config.BaseDir()
	if err != nil {
		return nil, err
	}

	pidsDir := filepath.Join(base, "pids")
	entries, err := os.ReadDir(pidsDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var sessions []Session
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".pid" {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".pid")
		pidPath := filepath.Join(pidsDir, e.Name())
		pid, live := daemon.TunnelAlreadyRunning(pidPath)
		sessions = append(sessions, Session{Name: name, PID: pid, Live: live})
	}
	return sessions, nil
}
