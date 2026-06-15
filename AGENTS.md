# AGENTS.md — Xray-core

## Quick Reference

```bash
# Build
CGO_ENABLED=0 go build -o xray -trimpath -ldflags "-s -w -buildid=" ./main

# Run all tests (no linter in CI)
go test -timeout 1h -v ./...

# Run a single package
go test -v ./proxy/vless/...

# Run scenario (integration) tests — builds a real xray binary as subprocess
go test -v ./testing/scenarios/...

# Regenerate all generated code
go generate ./core/...
# Then chacha20:
cd common/crypto/internal && go generate chacha.go && cd ../../..
```

## Architecture

### Core Lifecycle

```
main() → cmdRun → core.LoadConfig() → core.New(config) → instance.Start()
```

- **`core.Instance`** (`core/xray.go:82`) — the server. Holds a slice of `features.Feature`, resolves dependencies via reflection, manages Start/Close lifecycle.
- **`core.New(config)`** — creates Instance, iterates `config.App` to create features via `CreateObject`/`AddFeature`, adds essential defaults (local DNS, default policy, default router, noop stats), then adds inbound/outbound handlers.
- **`instance.Start()`** — calls `Start()` on every registered feature.
- **`instance.Close()`** — calls `Close()` on every feature.

### Feature System (`features/feature.go`)

```go
type Feature interface {
    common.HasType   // Type() interface{} — returns (*Type)(nil)
    common.Runnable  // Start() error + Close() error
}
```

Every feature lives in `app/` and implements `Feature`. Key feature interfaces in `features/`:

| Interface | File | Purpose |
|---|---|---|
| `inbound.Manager` | `features/inbound/` | Manage inbound proxy handlers |
| `outbound.Manager` | `features/outbound/` | Manage outbound proxy handlers |
| `routing.Router` | `features/routing/` | Pick outbound tag for a request |
| `routing.Dispatcher` | `features/routing/` | Dispatch to outbound, return Link |
| `dns.Client` | `features/dns/` | DNS resolution |
| `policy.Manager` | `features/policy/` | Connection policy |
| `stats.Manager` | `features/stats/` | Traffic statistics |

### Proxy Interfaces (`proxy/proxy.go`)

```go
type Inbound interface {
    Network() []net.Network
    Process(context.Context, net.Network, stat.Connection, routing.Dispatcher) error
}
type Outbound interface {
    Process(context.Context, *transport.Link, internet.Dialer) error
}
```

Proxies live in `proxy/`. Each registers via `common.RegisterConfig()` in `init()`.

### Dependency Injection (Reflection-Based)

Features declare dependencies by function parameter types:

```go
core.RequireFeatures(ctx, func(router routing.Router, dns dns.Client) error {
    // both guaranteed to exist when this runs
    return nil
})
```

- `RequireFeatures(callback, optional=false)` — panics if dep not found
- `OptionalFeatures(callback, optional=true)` — silently skips missing deps
- Resolution is deferred: if a dep isn't registered yet, the callback is queued and retried when new features are added via `AddFeature()`

## Registration Pattern (Critical)

Xray-core uses **Go `init()` + blank import** as its entire plugin system. There is no service locator, no YAML plugin config, no reflection-based discovery.

### The Composition Root

**`main/distro/all/all.go`** is the single file that wires everything together via 80+ blank imports:

```go
import (
    _ "github.com/xtls/xray-core/app/dispatcher"     // mandatory
    _ "github.com/xtls/xray-core/proxy/vless/inbound" // proxy
    _ "github.com/xtls/xray-core/transport/internet/tcp" // transport
    _ "github.com/xtls/xray-core/main/json"           // config format
    _ "github.com/xtls/xray-core/main/commands/all"   // CLI commands
)
```

Each imported package has an `init()` that calls `common.RegisterConfig()` or `core.RegisterConfigLoader()` or appends to `base.RootCommand.Commands`.

### Type Registry (`common/type.go`)

```go
var typeCreatorRegistry = make(map[reflect.Type]ConfigCreator)
func RegisterConfig(config interface{}, creator ConfigCreator) error
func CreateObject(ctx context.Context, config interface{}) (interface{}, error)
```

Every proxy, app, and transport registers a `ConfigCreator` keyed by its protobuf config type. `CreateObject()` looks up the creator by `reflect.TypeOf(config)` and calls it.

### `common.Must()` Pattern

```go
func Must(err error) { if err != nil { panic(err) } }
```

Used pervasively in `init()` functions for registration calls that cannot return errors. Also used in crypto setup and invariant checks. **Only for programmer errors, never for runtime I/O errors.**

## Config System

### Config Loading Chain

```
run.go → getConfigFilePath() → core.LoadConfig(format, files)
  → ConfigBuilderForFiles (set by infra/conf/serial/builder.go init)
    → For each file: decode by format → conf.Config.Override() merge
    → conf.Config.Build() → *core.Config (protobuf)
```

### Config File Resolution Order

1. `-confdir` flag (or `XRAY_LOCATION_CONFDIR` env)
2. `-config` / `-c` flags
3. `./config.json` (working directory)
4. `XRAY_LOCATION_CONFIG` env
5. `stdin:`

### Supported Formats

| Format | Extension | Decoder Path |
|--------|-----------|-------------|
| JSON   | `.json`, `.jsonc` | Direct → `json.Decoder` → `conf.Config` |
| YAML   | `.yaml`, `.yml` | YAML → JSON (ghodss/yaml) → `DecodeJSONConfig` |
| TOML   | `.toml` | TOML → map → JSON → `DecodeJSONConfig` |
| Protobuf | `.pb` | Direct `proto.Unmarshal` (single file only) |

JSON supports comments: `//`, `/* */`, `#` (via `infra/conf/json/reader.go`).

### Multi-File Merging (`conf.Config.Override()`)

- Scalar fields (Log, DNS, Routing): later file replaces earlier if non-nil
- Inbounds: matched by tag (update if exists, append if new)
- Outbounds: matched by tag (update if exists); filename containing "tail" → append, otherwise prepend

## Code Generation

All `go:generate` directives are in `core/` (3 files) and `common/crypto/internal/` (1 file). Generated files are **committed to the repo** — CI does not regenerate them.

### 1. Protobuf → Go (`core/proto.go`)

```bash
go generate ./core/proto.go
# Requires: protoc on PATH, protoc-gen-go, protoc-gen-go-grpc
# Produces: *.pb.go and *_grpc.pb.go alongside each .proto file
```

65 proto files across `proxy/`, `transport/`, `common/`, `app/`, `core/`.

### 2. Mocks (`core/mocks.go`)

```bash
go generate ./core/mocks.go
# Uses: github.com/golang/mock/mockgen
# Produces: testing/mocks/{io,log,mux,dns,outbound,proxy}.go
```

6 mock types: `Reader`, `Writer`, `LogHandler`, `MuxClientWorkerFactory`, `DNSClient`, `OutboundManager`, `OutboundHandlerSelector`, `ProxyInbound`, `ProxyOutbound`.

### 3. Code Formatting (`core/format.go`)

```bash
go generate ./core/format.go
# Uses: gofumpt + gci
# Formats all *.go files (excludes *.pb.go, testing/mocks/, main/distro/all/all.go)
```

### 4. ChaCha20 Block (`common/crypto/internal/chacha.go`)

```bash
cd common/crypto/internal && go generate chacha.go
# Produces: chacha_core.generated.go
```

### Note: No `errorgen`

There is no error code generator. Use `errors.New(msg...)` directly. The `common/errors/errorgen/` directory does not exist.

## Testing

### Two-Tier System

**Unit tests** — co-located with source (`*_test.go`). Use gomock mocks from `testing/mocks/`:

```go
mockCtl := gomock.NewController(t)
defer mockCtl.Finish()
mockDNS := mocks.NewDNSClient(mockCtl)
mockDNS.EXPECT().LookupIP(gomock.Any(), gomock.Any()).Return(...)
```

**Scenario (integration) tests** — in `testing/scenarios/` (20 files). Build a real xray binary, run it as a subprocess, test actual TCP/UDP proxy traffic with XOR round-trip verification:

```go
// Pattern:
servers, _ := InitializeServerConfigs(...)
defer CloseAllServers(servers)
testTCPConn(port, payloadSize, timeout)
```

### Test Servers

- `testing/servers/tcp/` — TCP echo with `MsgProcessor`, `PickPort()` for dynamic ports
- `testing/servers/udp/` — UDP echo
- `testing/servers/http/` — HTTP with path handlers

### Coverage

Build-tag controlled (`//go:build coverage` / `//go:build !coverage`). Scripts at `testing/coverage/coverall` and `coverall2`. Not wired into CI currently.

### CI (`test.yml`)

- Matrix: windows-latest, ubuntu-latest, macos-latest
- Command: `go test -timeout 1h -v ./...`
- **No linter, no `go vet`, no coverage, no race detector in CI**

## Key Patterns

### Error Handling (`common/errors/`)

```go
err := errors.New("something failed").Base(innerErr).AtWarning()
errors.LogInfo(ctx, "processing request for ", dest)
errors.Cause(err)  // unwrap to root
errors.Combine(err1, err2)  // aggregate
```

The `errors.Error` struct auto-captures the caller function name via `runtime.Caller(1)`. Severity levels: Debug, Info, Warning, Error. Use `.AtWarning()`, `.AtError()`, etc.

### Buffer I/O (`common/buf/`)

- **`Buffer`** — 8K recyclable byte slice from `sync.Pool`. Has `UDP *net.Destination` for per-packet routing.
- **`MultiBuffer`** — `[]*Buffer`, the fundamental I/O unit.
- **`buf.Reader`** / **`buf.Writer`** — interfaces with `ReadMultiBuffer()` / `WriteMultiBuffer()`.
- **`buf.Copy(reader, writer, opts...)`** — full-duplex copy with activity tracking, byte counting, stats.
- On Linux, TCP connections use `readv` syscall for coalesced reads.

### Transport Layer (`transport/`)

```
transport.Link { Reader, Writer }  ← connects inbound ↔ outbound
  ↓
transport/internet/
  Dial(ctx, dest, streamSettings) → stat.Connection
  ListenTCP(ctx, addr, port, settings, handler)
  Registry: protocol → dialFunc / ListenFunc
```

Each transport protocol (TCP, WebSocket, gRPC, KCP, etc.) registers dialer + listener in `init()`. Security layers (TLS, REALITY) are applied as wrappers.

### `common/singbridge/`

Bridges Xray-core types to/from sing-box (`github.com/sagernet/sing`) types. Used by TUN inbound and any feature using sing-box networking primitives. Converts destinations, dialers, connection handlers, and loggers between the two ecosystems.

### Platform (`common/platform/`)

OS-conditional builds via file suffixes (`windows.go`, `others.go`). Environment flags read via `xray.*` naming convention (e.g., `xray.location.asset` → `XRAY_LOCATION_ASSET`). Key env vars: `xray.buf.readv`, `xray.buf.splice`, `xray.cone.disabled`.

### UUID (`common/uuid/`)

`UUID [16]byte`. `New()` creates RFC 4122 v4. `ParseString()` has a quirk: short strings (< 32 chars) produce a **v5-style SHA-1 deterministic UUID** from the zero UUID + input — used for VMess user ID generation.

## Common Gotchas

1. **No linter in CI** — run `go vet ./...` manually before pushing.
2. **Generated files are committed** — if you change a `.proto` file, you must regenerate and commit the `.pb.go` files.
3. **`init()` ordering matters** — `main/distro/all/all.go` blank imports must include every package that registers itself. Missing an import = feature silently unavailable.
4. **`common.Must()` panics** — only use for programmer errors (registration, crypto setup). Never for I/O or network errors.
5. **Scenario tests build a binary** — they call `go build` as a subprocess. A broken build will fail all scenario tests.
6. **Config is protobuf internally** — JSON/YAML/TOML are converted to `*core.Config` (protobuf) before use. The `conf.Config` (in `infra/conf/`) is the intermediate JSON-shaped struct.
7. **Protobuf config is single-file only** — `core.LoadConfig()` rejects multiple `.pb` files.
8. **`xray run` is the default** — running `xray` with no subcommand is equivalent to `xray run` (backward compat with v2fly).
9. **Port allocation in tests** — `tcp.PickPort()` uses listen-then-close, which is not race-free. Tests could theoretically collide.
10. **Windows 7 builds** use Go 1.21.4 toolchain (all others use 1.23).
