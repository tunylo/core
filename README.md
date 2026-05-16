# tunylo/core

Core library for [tunylo](https://github.com/tunylo/cli). Provides the shared packages used by all tunylo clients.

## Packages

| Package | Description |
|---------|-------------|
| `config` | Config loading, saving, and platform-aware paths (including named PID/log paths for background tunnels) |
| `tunnels` | Tunnel interface, registry, and provider implementations |
| `server` | Local reverse proxy with passcode access control and request logging |

## Install

```bash
go get github.com/tunylo/core
```

## Usage

```go
import (
    "github.com/tunylo/core/config"
    "github.com/tunylo/core/tunnels"
    "github.com/tunylo/core/server"
)
```

### config

```go
cfg, err := config.Load()

pidPath, err := config.NamedPidPath("myapp")
logPath, err := config.NamedLogPath("myapp")
```

Platform paths:

| OS | Base directory |
|----|----------------|
| macOS | `~/Library/Application Support/tunylo/` |
| Linux / other | `~/.config/tunylo/` |

Named PID files are stored under `<base>/pids/<name>.pid` and log files under `<base>/logs/<name>.log`.

### tunnels

```go
for _, t := range tunnels.AllTunnels() {
    fmt.Println(t.Key(), t.Label(), t.IsInstalled())
}

t := tunnels.AllTunnels()[0]
proc, err := t.Start("localhost", 3000)
fmt.Println(*proc.PublicURL)
```

### server

```go
p, err := server.New("mysecret", "localhost", 3000)
port, err := p.Start(ctx)
```

Starts a local reverse proxy on a random port. Incoming requests are gated behind a passcode unlock page. Failed attempts are rate-limited per IP.

## License

MIT
