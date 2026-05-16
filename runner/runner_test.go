package runner

import (
	"context"
	"strings"
	"testing"
)

func TestRunUnknownTunnel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	_, err := Run(context.Background(), Options{
		Host:   "localhost",
		Port:   8080,
		Tunnel: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for unknown tunnel")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunNoTunnelConfigured(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	_, err := Run(context.Background(), Options{
		Host: "localhost",
		Port: 8080,
	})
	if err == nil {
		t.Fatal("expected error when no tunnel configured")
	}
	if !strings.Contains(err.Error(), "no tunnel") {
		t.Fatalf("unexpected error: %v", err)
	}
}
