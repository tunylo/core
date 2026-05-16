package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/tunylo/core/config"
)

type SpawnResult struct {
	PID     int
	LogPath string
	PidPath string
}

func TunnelAlreadyRunning(pidPath string) (int, bool) {
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, false
	}
	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil || pid <= 0 {
		return 0, false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return 0, false
	}
	return pid, proc.Signal(syscall.Signal(0)) == nil
}

func Spawn(name, exe string, args []string) (*SpawnResult, error) {
	pidPath, err := config.NamedPidPath(name)
	if err != nil {
		return nil, err
	}

	if pid, running := TunnelAlreadyRunning(pidPath); running {
		return nil, fmt.Errorf("a tunnel named %q is already running (PID %d)", name, pid)
	}

	logPath, err := config.NamedLogPath(name)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(pidPath), 0o755); err != nil {
		return nil, err
	}

	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("creating log file: %w", err)
	}

	cmd := exec.Command(exe, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("starting background process: %w", err)
	}

	os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0o644) //nolint:errcheck

	return &SpawnResult{
		PID:     cmd.Process.Pid,
		LogPath: logPath,
		PidPath: pidPath,
	}, nil
}

func WaitForURL(logPath string, timeout time.Duration) (string, bool) {
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(logPath)
		if err == nil {
			clean := ansi.ReplaceAllString(string(data), "")
			for _, line := range strings.Split(clean, "\n") {
				for _, field := range strings.Fields(line) {
					if strings.HasPrefix(field, "https://") {
						return field, true
					}
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return "", false
}
