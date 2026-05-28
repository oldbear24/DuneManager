# Copilot Instructions — DuneManager

## What this project is

DuneManager manages a **Dune Awakening dedicated server** running inside a Hyper-V VM on Windows. It is split into three binaries built from a single Go module:

| Binary | Entry point | Notes |
|---|---|---|
| `dune-manager.exe` | `main.go` (root) | Fyne GUI; no elevation required |
| `dune-manager-svc.exe` | `cmd/service/main.go` | HTTP service; needs elevation for Hyper-V |
| `dune-manager-updater.exe` | `cmd/updater/main.go` | Self-update helper; runs detached |

## Build commands

`CGO_ENABLED=1` is required (Fyne uses CGO). MinGW must be on `PATH` on Windows.

```powershell
# GUI (no console window)
go build -ldflags "-H windowsgui -X github.com/oldbear24/DuneManager/internal/build.Version=v1.0.0" -o dune-manager.exe .

# Background service
go build -ldflags "-X github.com/oldbear24/DuneManager/internal/build.Version=v1.0.0" -o dune-manager-svc.exe ./cmd/service/

# Updater helper
go build -o dune-manager-updater.exe ./cmd/updater/
```

Validate without producing binaries:
```powershell
go build ./...
go test ./...
```

Run a single package's tests:
```powershell
go test ./internal/config/...
```

## Architecture

```
dune-manager.exe (Fyne GUI)
        │  HTTP on 127.0.0.1:7374 (configurable)
        ▼
dune-manager-svc.exe (net/http server)
        │
        ├─ Hyper-V ──► PowerShell (Start-VM / Stop-VM / vm-utilities.ps1)
        ├─ SSH ──────► dune@<vm-ip>  (battlegroup CLI, kubectl, password change)
        └─ Updater ──► dune-manager-updater.exe (detached helper process)
```

- **GUI never calls Hyper-V or SSH directly.** It calls `api.Client`, which POSTs to `/api/exec` with a named command string.
- **Service is the single point of truth.** All state (VM status, busy flag, active kill func) lives in `api.Server`.
- **Only one command runs at a time.** `beginExclusive` / `endExclusive` serialize execution; `/api/kill` can cancel it.
- **All command output is streamed as Server-Sent Events** (`text/event-stream`). Each event is `data: {"type":"output"|"done","line":"...","error":"..."}`.
- **Version is injected at build time** via `-ldflags "-X …/build.Version=…"`. The fallback is `"dev"`. `"dev"` builds never trigger auto-update.

## Key packages

| Package | Responsibility |
|---|---|
| `internal/api` | HTTP server (`server.go`), HTTP client (`client.go`), shared types (`types.go`) |
| `internal/runner` | Run external processes (PowerShell, SSH) with streaming stdout/stderr; `HideWindow: true` on every process |
| `internal/config` | JSON config in `dune-manager.json` next to the executable; thread-safe `Get()` / `Set()` / `Save()` |
| `internal/vm` | Hyper-V VM state via PowerShell (`Get-VM`, `Get-VMNetworkAdapter`) |
| `internal/winsvc` | Windows SCM integration (install, start, stop, uninstall service named `"DuneManager"`) |
| `internal/updater` | GitHub Releases API check + binary replace + `LaunchHelper` for detached update plan |
| `internal/discord` | Optional Discord bot using `discordgo`; disabled when `DiscordToken` is empty |
| `internal/build` | Single `Version` variable set by ldflags |
| `internal/logging` | Shared logger (file + optional Event Viewer) |
| `internal/ui` | Fyne application (`app.go`) |

## Key conventions

### Adding a new command

Commands are named strings dispatched in `handleExec` in `internal/api/server.go`. To add one:
1. Add a `case "my-cmd":` in the `switch req.Cmd` block.
2. Implement an `execMyCmd(out func(string), ...)` method on `*Server`.
3. Use `s.execPS(script, out)` for PowerShell or `s.execSSH(cfg, state, remoteCmd, out)` for SSH.
4. Add a corresponding button / call in the Fyne UI (`internal/ui/app.go`) via `client.Exec(api.ExecRequest{Cmd: "my-cmd"}, onLine)`.

### Running external processes

Always go through `internal/runner`:
- `runner.RunInteractive` — used by `execCmd` / `execCmdWithOptions` in the server for killable streaming commands.
- `runner.RunPS` / `runner.RunSSHCmd` — simpler wrappers for fire-and-collect use.
- All processes set `SysProcAttr{HideWindow: true}` to suppress console pop-ups.
- Battlegroup SSH commands must use `TERM=dumb … 2>&1` and pass output through `stripANSI()` to suppress escape codes.

### Config access

- Always read config via `config.Get()` (returns a snapshot; safe for concurrent use).
- `config.VMUtilitiesPS()` returns the path to `vm-utilities.ps1` inside `scriptsDir`.
- Config file is stored next to the service executable as `dune-manager.json`.

### Service management

- The service name constant is `winsvc.ServiceName` (`"DuneManager"`).
- Service restarts / updates go through `updater.LaunchHelper` with a `HelperPlan` — the helper waits for the current PID to exit, replaces binaries, then starts the service via SCM.
- Do not restart the service in-process; always use the SCM path through `winsvc` or the helper.

### SSE pattern

Server endpoints that stream output set `Content-Type: text/event-stream` and emit lines as:
```
data: {"type":"output","line":"...\n"}

data: {"type":"done","line":"<optional result>","error":"<on failure>"}

```
The client in `api/client.go` (`Exec`, `ApplyServiceUpdate`) uses a `bufio.Scanner` to parse these.

## Release process

Releases are built by `.github/workflows/release.yml` on `release: published`. The workflow runs on `windows-latest` with `CGO_ENABLED=1` and MinGW. Release assets must be named exactly `dune-manager.exe`, `dune-manager-svc.exe`, and `dune-manager-updater.exe` — the auto-update logic in `internal/updater` matches on these exact filenames.
