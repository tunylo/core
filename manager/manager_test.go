package manager

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tunylo/core/config"
)

func TestStopNamedMissingPidFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	_, err := StopNamed("ghost")
	if err == nil {
		t.Fatal("expected error for missing pid file")
	}
}

func TestStopNamedInvalidPidContent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	pidPath, _ := config.NamedPidPath("bad")
	os.MkdirAll(filepath.Dir(pidPath), 0o755)
	os.WriteFile(pidPath, []byte("notanumber"), 0o644)

	_, err := StopNamed("bad")
	if err == nil {
		t.Fatal("expected error for invalid PID content")
	}
}

func TestStopNamedRemovesPidFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	proc := exec.Command("sleep", "1000")
	if err := proc.Start(); err != nil {
		t.Fatalf("could not start sleep: %v", err)
	}
	t.Cleanup(func() { proc.Process.Kill(); proc.Wait() })

	pidPath, _ := config.NamedPidPath("sleeper")
	os.MkdirAll(filepath.Dir(pidPath), 0o755)
	os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", proc.Process.Pid)), 0o644)

	pid, err := StopNamed("sleeper")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pid != proc.Process.Pid {
		t.Fatalf("expected PID %d, got %d", proc.Process.Pid, pid)
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatal("expected pid file to be removed")
	}
}

func TestListRunning_Empty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	sessions, err := ListRunning()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestListRunning_WithLiveSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	pidPath, _ := config.NamedPidPath("live")
	os.MkdirAll(filepath.Dir(pidPath), 0o755)
	os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0o644)

	sessions, err := ListRunning()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Name != "live" {
		t.Fatalf("expected name 'live', got %s", sessions[0].Name)
	}
	if !sessions[0].Live {
		t.Fatal("expected session to be live")
	}
}
