# tunylo/core

Core library for [tunylo](https://github.com/tunylo/cli). Provides the shared packages used by all tunylo clients.

## Packages

| Package | Description |
|---------|-------------|
| `config` | Config loading, saving, and platform-aware paths |
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

## License

MIT
