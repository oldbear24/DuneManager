# Dune Manager

A Windows application for managing a **Dune Awakening dedicated server** running inside a Hyper-V virtual machine.

The project is split into two binaries:

| Binary | Role |
|---|---|
| `dune-manager-svc.exe` | Background HTTP service — runs all Hyper-V / SSH operations. Can run as a Windows service, a Task Scheduler task, or in a console with `--run`. |
| `dune-manager.exe` | GUI client — connects to the service via HTTP. **No elevation required.** |
| `dune-manager-updater.exe` | Helper used during self-update so running binaries can be replaced safely. |

---

## Features

- One-click VM start / stop
- Battlegroup management (status, start, stop, restart, update, swap, backup)
- In-app streaming output — no console pop-ups
- File browser and Director web UI shortcuts
- ⚙ Settings dialog — configure port, VM name, scripts dir, SSH key path, GitHub repo
- ⏹ Kill button — cancel a running command from the GUI
- Discord bot — control the service via `/dune` slash commands (optional)
- 🔄 Auto-update — manual check with one-click update for both binaries

---

## Prerequisites

| Requirement | Notes |
|---|---|
| Windows 10/11 Pro or Enterprise | Hyper-V must be available |
| Hyper-V enabled | `Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V-All` |
| OpenSSH client | Settings → Optional Features → OpenSSH Client |
| Go 1.21+ | For building from source |

---

## Building from Source

```powershell
git clone <repo-url> DuneManager
cd DuneManager
go mod download

# GUI binary (no console window)
go build -ldflags "-H windowsgui -X github.com/oldbear24/DuneManager/internal/build.Version=v1.0.0" -o dune-manager.exe .

# Background service binary
go build -ldflags "-X github.com/oldbear24/DuneManager/internal/build.Version=v1.0.0" -o dune-manager-svc.exe ./cmd/service/

# Updater helper binary
go build -o dune-manager-updater.exe ./cmd/updater/
```

---

## Directory Layout

```
dune-manager.exe          ← GUI client
dune-manager-svc.exe      ← Background service
dune-manager-updater.exe  ← Self-update helper
dune-manager.json         ← Config file (auto-created on first save)
bats\
  battlegroup-management\
    vm-utilities.ps1
    ...
```

---

## Configuration

On first launch the service creates `dune-manager.json` next to the executable.
You can also edit it via **⚙ Settings** in the GUI.

```json
{
  "port": 7374,
  "vmName": "dune-awakening",
  "scriptsDir": "C:\\Tools\\DuneManager\\bats\\battlegroup-management",
  "sshKeyPath": "C:\\Users\\you\\AppData\\Local\\DuneAwakeningServer\\sshKey"
}
```

| Field | Description | Default |
|---|---|---|
| `port` | Port the service listens on | `7374` |
| `vmName` | Hyper-V VM name | `dune-awakening` |
| `scriptsDir` | Path to the `battlegroup-management` scripts folder | `<exe dir>\bats\battlegroup-management` |
| `sshKeyPath` | Path to the SSH private key for the VM | `%LOCALAPPDATA%\DuneAwakeningServer\sshKey` |
| `discordToken` | Discord bot token — leave empty to disable | _(disabled)_ |
| `discordGuildID` | Guild ID for instant slash-command registration | _(global)_ |
| `discordChannelID` | Restrict bot commands to one channel | _(any channel)_ |
| `discordRoleID` | Restrict bot commands to members with one Discord role | _(any role)_ |
| `githubRepo` | `owner/repo` for GitHub release auto-update checks | _(disabled)_ |

> If you change the port, restart the service for it to take effect.

---

## Auto-Update

When `githubRepo` is set (e.g. `"alice/dune-manager"`) the GUI can check for a newer GitHub Release from the **🔍 Check updates** button.

If an update is available an **⬆ Update vX.Y.Z** button appears in the status bar. Clicking it will:

1. Ask for confirmation.
2. Download and apply the new **service binary** (the service self-restarts).
3. Download and apply the new **GUI binary** (the GUI self-restarts).

**GitHub Release asset naming convention** — your release must contain assets with exactly these names:

| Asset name | Binary |
|---|---|
| `dune-manager-svc.exe` | Background service |
| `dune-manager.exe` | GUI client |
| `dune-manager-updater.exe` | Self-update helper |

**Build releases with the version injected:**

```powershell
$VERSION = "v1.2.0"
go build -ldflags "-H windowsgui -X github.com/oldbear24/DuneManager/internal/build.Version=$VERSION" -o dune-manager.exe .
go build -ldflags "-X github.com/oldbear24/DuneManager/internal/build.Version=$VERSION" -o dune-manager-svc.exe ./cmd/service/
go build -o dune-manager-updater.exe ./cmd/updater/
```

---

## Running

### Quick start (manual / testing)

Open an **elevated** PowerShell window (the service needs Hyper-V access):

```powershell
cd C:\Tools\DuneManager
.\dune-manager-svc.exe --run
```

Then launch the GUI (no elevation needed):

```powershell
.\dune-manager.exe
```

The GUI will show **Service Offline** in the status bar if the service is not running.

---

## Running the Service Automatically at Startup

### Option A — Windows Service (Recommended)

The service binary has built-in service management commands.
Run these once in an **elevated** PowerShell:

```powershell
$svcExe = "C:\Tools\DuneManager\dune-manager-svc.exe"

# Install and enable the service
& $svcExe --install
Set-Service -Name DuneManager -StartupType Automatic

# Start it now
& $svcExe --start
```

The service runs as `LocalSystem` (has Hyper-V access) and starts automatically at boot.

The service writes logs to `dune-manager-svc.log` next to the executable. When installed as a Windows service, warnings and errors are also written to **Windows Event Viewer**.

**Other service commands:**

```powershell
.\dune-manager-svc.exe --stop       # stop the service
.\dune-manager-svc.exe --uninstall  # remove the service
```

**Add the GUI to logon startup** (no elevation needed):

```powershell
$guiExe = "C:\Tools\DuneManager\dune-manager.exe"
$wsh = New-Object -ComObject WScript.Shell
$shortcut = $wsh.CreateShortcut("$env:APPDATA\Microsoft\Windows\Start Menu\Programs\Startup\DuneManager.lnk")
$shortcut.TargetPath = $guiExe
$shortcut.WorkingDirectory = Split-Path $guiExe
$shortcut.Save()
```

---

### Option B — Task Scheduler (no service install)

Use this if you prefer Task Scheduler over a Windows service.

```powershell
$svcExe = "C:\Tools\DuneManager\dune-manager-svc.exe"

$action    = New-ScheduledTaskAction -Execute $svcExe -Argument "--run" `
                 -WorkingDirectory (Split-Path $svcExe)
$trigger   = New-ScheduledTaskTrigger -AtStartup
$principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -RunLevel Highest
$settings  = New-ScheduledTaskSettingsSet `
                 -ExecutionTimeLimit 0 `
                 -MultipleInstances IgnoreNew `
                 -AllowStartIfOnBatteries `
                 -DontStopIfGoingOnBatteries

Register-ScheduledTask `
    -TaskName "Dune Manager Service" `
    -Action $action -Trigger $trigger `
    -Principal $principal -Settings $settings -Force
```

Then add the GUI to your user's logon startup as shown in Option A.

---

## Troubleshooting

| Symptom | Fix |
|---|---|
| GUI shows "Service Offline" | Start the service: `.\dune-manager-svc.exe --run` or `.\dune-manager-svc.exe --start` |
| Service fails to start | Make sure the port (default 7374) is not in use. Check with `netstat -ano \| findstr 7374` |
| Need service logs | Check `dune-manager-svc.log` next to `dune-manager-svc.exe`, and use Event Viewer for service warnings/errors |
| VM state shows "error" | Hyper-V PowerShell module may not be loaded. Run `Import-Module Hyper-V` and retry. |
| SSH commands fail | Verify the SSH key path in ⚙ Settings and that the VM IP is reachable. |
| Port conflict | Change the port in ⚙ Settings (GUI), save, then restart the service. |
| Settings not saved | The config file is written next to the service executable. Make sure the path is writable. |
