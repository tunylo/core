# tunylo/core

Core library for [tunylo](https://github.com/tunylo/cli). Provides all shared logic for tunylo clients — CLI, desktop app, or any other consumer.

## Packages

| Package | Description |
|---------|-------------|
| `runner` | Resolve tunnel, auto-install binary, start passcode proxy, start tunnel — returns public URL |
| `daemon` | Spawn a detached background process, write PID file, poll log file for public URL |
| `manager` | Stop a named background tunnel, list all running sessions |
| `config` | Config loading/saving and platform-aware paths (PID files, log files, bin dir) |
| `tunnels` | Tunnel interface, registry, and provider implementations |
| `server` | Local reverse proxy with passcode access control and request logging |

## Install

```bash
go get github.com/tunylo/core
```

## Usage

### runner

The main entry point for starting a tunnel. Handles tunnel resolution, auto-install, optional passcode proxy, and process startup. Returns a `*Result` with the public URL and process handle.

```go
import "github.com/tunylo/core/runner"

res, err := runner.Run(ctx, runner.Options{
    Host:     "localhost",
    Port:     3000,
    Tunnel:   "cloudflare",
    Passcode: "mysecret",
    OnInstall: func(label string) {
        fmt.Printf("Installing %s...\n", label)
    },
    OnBeforeStart: func(label string) {
        fmt.Printf("Starting %s...\n", label)
    },
})
if err != nil {
    log.Fatal(err)
}
fmt.Println(*res.PublicURL)
defer res.Cmd.Process.Kill()
```

### daemon

Spawns the current binary as a detached background process and writes a named PID file. `WaitForURL` polls the log file until the public URL appears.

```go
import "github.com/tunylo/core/daemon"

result, err := daemon.Spawn("myapp", exe, args)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("PID: %d\nLog: %s\n", result.PID, result.LogPath)

url, ok := daemon.WaitForURL(result.LogPath, 45*time.Second)
if ok {
    fmt.Println("URL:", url)
}
```

`daemon.TunnelAlreadyRunning(pidPath)` checks whether a PID file points to a live process.

### manager

Stops a named background tunnel and can enumerate all running sessions.

```go
import "github.com/tunylo/core/manager"

pid, err := manager.StopNamed("myapp")

sessions, err := manager.ListRunning()
for _, s := range sessions {
    fmt.Printf("%s  PID=%d  live=%v\n", s.Name, s.PID, s.Live)
}
```

### config

```go
import "github.com/tunylo/core/config"

cfg, err := config.Load()
config.Save(&config.Config{DefaultTunnel: &key})

pidPath, _ := config.NamedPidPath("myapp")
logPath, _ := config.NamedLogPath("myapp")
base,    _ := config.BaseDir()
```

Platform base directories:

| OS | Path |
|----|------|
| macOS | `~/Library/Application Support/tunylo/` |
| Linux / other | `~/.config/tunylo/` |

Named PID files: `<base>/pids/<name>.pid` — Log files: `<base>/logs/<name>.log`

### tunnels

```go
import "github.com/tunylo/core/tunnels"

for _, t := range tunnels.AllTunnels() {
    fmt.Println(t.Key(), t.Label(), t.IsInstalled())
}

proc, err := tunnels.AllTunnels()[0].Start("localhost", 3000)
fmt.Println(*proc.PublicURL)
```

### server

Starts a local reverse proxy on a random port. All requests are gated behind a passcode unlock page. Failed attempts are rate-limited per IP.

```go
import "github.com/tunylo/core/server"

p, err := server.New("mysecret", "localhost", 3000)
port, err := p.Start(ctx)
```

## License

MIT
