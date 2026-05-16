package runner

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/tunylo/core/config"
	"github.com/tunylo/core/server"
	"github.com/tunylo/core/tunnels"
)

type Options struct {
	Host          string
	Port          uint16
	Tunnel        string
	Passcode      string
	OnInstall     func(label string)
	OnBeforeStart func(label string)
}

type Result struct {
	PublicURL   *string
	TunnelLabel string
	ProxyPort   int
	Cmd         *exec.Cmd
}

func Run(ctx context.Context, opts Options) (*Result, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	key := opts.Tunnel
	if key == "" && cfg.DefaultTunnel != nil {
		key = *cfg.DefaultTunnel
	}
	if key == "" {
		return nil, fmt.Errorf("no tunnel selected")
	}

	var t tunnels.Tunnel
	for _, tt := range tunnels.AllTunnels() {
		if tt.Key() == key {
			t = tt
			break
		}
	}
	if t == nil {
		return nil, fmt.Errorf("unknown tunnel: %s", key)
	}

	if !t.IsInstalled() {
		if opts.OnInstall != nil {
			opts.OnInstall(t.Label())
		}
		if err := t.Install(); err != nil {
			return nil, err
		}
	}

	tunnelHost := opts.Host
	tunnelPort := opts.Port
	var proxyPort int

	if opts.Passcode != "" {
		p, err := server.New(opts.Passcode, opts.Host, opts.Port)
		if err != nil {
			return nil, fmt.Errorf("creating proxy: %w", err)
		}
		proxyPort, err = p.Start(ctx)
		if err != nil {
			return nil, fmt.Errorf("starting proxy: %w", err)
		}
		tunnelHost = "127.0.0.1"
		tunnelPort = uint16(proxyPort)
	}

	if opts.OnBeforeStart != nil {
		opts.OnBeforeStart(t.Label())
	}

	proc, err := t.Start(tunnelHost, tunnelPort)
	if err != nil {
		return nil, err
	}

	return &Result{
		PublicURL:   proc.PublicURL,
		TunnelLabel: t.Label(),
		ProxyPort:   proxyPort,
		Cmd:         proc.Cmd,
	}, nil
}
