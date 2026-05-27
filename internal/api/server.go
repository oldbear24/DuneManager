package api

import (
"context"
"encoding/json"
"fmt"
"net/http"
"os"
"os/exec"
"strings"
"sync"
"syscall"
"time"

"github.com/oldbear24/DuneManager/internal/build"
"github.com/oldbear24/DuneManager/internal/config"
"github.com/oldbear24/DuneManager/internal/runner"
"github.com/oldbear24/DuneManager/internal/updater"
"github.com/oldbear24/DuneManager/internal/vm"
)

// Server wraps the HTTP service that runs in the background process.
type Server struct {
httpServer *http.Server
mu         sync.Mutex
busy       bool
activeKill func()
}

// NewServer builds the HTTP mux and server struct but does not start listening.
func NewServer() *Server {
s := &Server{}
mux := http.NewServeMux()
mux.HandleFunc("/api/status", s.handleStatus)
mux.HandleFunc("/api/exec", s.handleExec)
mux.HandleFunc("/api/kill", s.handleKill)
mux.HandleFunc("/api/version", s.handleVersion)
mux.HandleFunc("/api/update/check", s.handleUpdateCheck)
mux.HandleFunc("/api/update/apply", s.handleUpdateApply)
s.httpServer = &http.Server{
Addr:    config.ServiceAddr(),
Handler: mux,
}
return s
}

// ListenAndServe starts the HTTP server (blocks until shutdown).
func (s *Server) ListenAndServe() error { return s.httpServer.ListenAndServe() }

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error { return s.httpServer.Shutdown(ctx) }

// handleStatus returns current VM state + busy flag.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
cfg := config.Get()
state, err := vm.GetState(cfg.VMName)
if err != nil {
state = &vm.State{VMState: "error"}
}
s.mu.Lock()
busy := s.busy
s.mu.Unlock()

w.Header().Set("Content-Type", "application/json")
_ = json.NewEncoder(w).Encode(StatusResponse{
Exists:  state.Exists,
Running: state.Running,
VMState: state.VMState,
IP:      state.IP,
Busy:    busy,
})
}

// handleExec runs a named command and streams output as Server-Sent Events.
func (s *Server) handleExec(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodPost {
http.Error(w, "POST required", http.StatusMethodNotAllowed)
return
}

var req ExecRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
http.Error(w, "invalid JSON", http.StatusBadRequest)
return
}

w.Header().Set("Content-Type", "text/event-stream")
w.Header().Set("Cache-Control", "no-cache")
w.Header().Set("Connection", "keep-alive")

flush := func() {
if f, ok := w.(http.Flusher); ok {
f.Flush()
}
}

s.mu.Lock()
if s.busy {
s.mu.Unlock()
writeSSE(w, SSEEvent{Type: "done", Error: "busy: another command is running"})
flush()
return
}
s.busy = true
s.mu.Unlock()

defer func() {
s.mu.Lock()
s.busy = false
s.activeKill = nil
s.mu.Unlock()
}()

// If the client disconnects, kill the active process.
ctx := r.Context()
abortDone := make(chan struct{})
go func() {
defer close(abortDone)
select {
case <-ctx.Done():
s.mu.Lock()
k := s.activeKill
s.mu.Unlock()
if k != nil {
k()
}
case <-abortDone:
}
}()

out := func(line string) {
writeSSE(w, SSEEvent{Type: "output", Line: line})
flush()
}

cfg := config.Get()
state, _ := vm.GetState(cfg.VMName)
if state == nil {
state = &vm.State{}
}

var result string
var execErr error

switch req.Cmd {
case "vm-start":
execErr = s.execVMStart(out, cfg)
case "vm-stop":
execErr = s.execVMStop(out, cfg)
case "ssh-rotate":
execErr = s.execSSHRotate(out, cfg, state)
case "password-change":
execErr = s.execPasswordChange(out, cfg, state, req.Password)
case "bg-status":
execErr = s.execSSH(cfg, state, "/home/dune/.dune/bin/battlegroup status", out)
case "bg-start":
execErr = s.execSSH(cfg, state, "/home/dune/.dune/bin/battlegroup start", out)
case "bg-stop":
execErr = s.execSSH(cfg, state, "/home/dune/.dune/bin/battlegroup stop", out)
case "bg-restart":
execErr = s.execSSH(cfg, state, "/home/dune/.dune/bin/battlegroup restart", out)
case "bg-update":
execErr = s.execSSH(cfg, state, "/home/dune/.dune/bin/battlegroup update", out)
case "bg-backup":
execErr = s.execSSH(cfg, state, "/home/dune/.dune/bin/battlegroup backup", out)
case "bg-swap":
execErr = s.execSSH(cfg, state, "/home/dune/.dune/bin/battlegroup enable-experimental-swap", out)
case "director-port":
result, execErr = s.execDirectorPort(out, cfg, state)
default:
execErr = fmt.Errorf("unknown command: %s", req.Cmd)
}

if execErr != nil {
writeSSE(w, SSEEvent{Type: "done", Error: execErr.Error()})
} else {
writeSSE(w, SSEEvent{Type: "done", Line: result})
}
flush()
}

// handleKill forcibly terminates the active command process.
func (s *Server) handleKill(w http.ResponseWriter, r *http.Request) {
s.mu.Lock()
k := s.activeKill
s.mu.Unlock()
if k != nil {
k()
}
w.WriteHeader(http.StatusNoContent)
}

func writeSSE(w http.ResponseWriter, evt SSEEvent) {
data, _ := json.Marshal(evt)
fmt.Fprintf(w, "data: %s\n\n", data)
}

// execCmd starts args, registers the kill func, and blocks until done.
func (s *Server) execCmd(args []string, out func(string)) error {
done := make(chan error, 1)
stdin, kill, err := runner.RunInteractive(args, out, func(e error) { done <- e })
if err != nil {
return err
}
_ = stdin.Close()
s.mu.Lock()
s.activeKill = kill
s.mu.Unlock()
return <-done
}

func (s *Server) execPS(script string, out func(string)) error {
return s.execCmd(
[]string{"powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script},
out,
)
}

func (s *Server) execSSH(cfg config.File, state *vm.State, remoteCmd string, out func(string)) error {
if !state.Running || state.IP == "" {
return fmt.Errorf("VM is not running or IP is unavailable")
}
return s.execCmd(
[]string{"ssh",
"-o", "StrictHostKeyChecking=no",
"-o", "LogLevel=QUIET",
"-i", cfg.SSHKeyPath,
fmt.Sprintf("dune@%s", state.IP),
remoteCmd,
},
out,
)
}

func (s *Server) execVMStart(out func(string), cfg config.File) error {
return s.execPS(fmt.Sprintf(`
$vmName = '%s'
$vm = Get-VM -Name $vmName -ErrorAction SilentlyContinue
if (-not $vm) { Write-Host "VM '$vmName' does not exist."; exit 1 }
if ($vm.State -eq 'Running') { Write-Host "VM is already running."; exit 0 }

Write-Host "Starting VM '$vmName'..." -ForegroundColor Cyan
Start-VM -Name $vmName

$timeout = 120; $elapsed = 0
do {
    Start-Sleep -Seconds 2; $elapsed += 2
    $vm = Get-VM -Name $vmName
} while ($vm.State -ne 'Running' -and $elapsed -lt $timeout)

if ($vm.State -eq 'Running') {
    Write-Host "VM started." -ForegroundColor Green
    $ip = $null; $elapsed = 0
    while (-not $ip -and $elapsed -lt 60) {
        Start-Sleep -Seconds 2; $elapsed += 2
        $ip = (Get-VMNetworkAdapter -VMName $vmName).IPAddresses |
              Where-Object { $_ -match '^\d+\.\d+\.\d+\.\d+$' } | Select-Object -First 1
    }
    if ($ip) { Write-Host "VM ready at $ip." -ForegroundColor Green }
    else { Write-Host "VM started - IP not yet assigned." -ForegroundColor Yellow }
} else {
    Write-Host "VM did not reach Running state in time." -ForegroundColor Red
    exit 1
}
`, cfg.VMName), out)
}

func (s *Server) execVMStop(out func(string), cfg config.File) error {
return s.execPS(fmt.Sprintf(`
Write-Host "Stopping VM '%s'..." -ForegroundColor Cyan
Stop-VM -Name '%s' -Force
Write-Host "VM stopped." -ForegroundColor Green
`, cfg.VMName, cfg.VMName), out)
}

func (s *Server) execSSHRotate(out func(string), cfg config.File, state *vm.State) error {
if !state.Running || state.IP == "" {
return fmt.Errorf("VM is not running or IP is unavailable")
}
return s.execPS(fmt.Sprintf(`. '%s'
Update-SshKey -Ip '%s'
`, config.VMUtilitiesPS(), state.IP), out)
}

func (s *Server) execPasswordChange(out func(string), cfg config.File, state *vm.State, password string) error {
if !state.Running || state.IP == "" {
return fmt.Errorf("VM is not running or IP is unavailable")
}
if password == "" {
return fmt.Errorf("password cannot be empty")
}
escaped := strings.ReplaceAll(password, "'", "''")
return s.execPS(fmt.Sprintf(`. '%s'
$pw = ConvertTo-SecureString '%s' -AsPlainText -Force
if (Set-VmPassword -Ip '%s' -NewPassword $pw) {
    Write-Host "Password changed successfully." -ForegroundColor Green
}
`, config.VMUtilitiesPS(), escaped, state.IP), out)
}

func (s *Server) execDirectorPort(out func(string), cfg config.File, state *vm.State) (string, error) {
if !state.Running || state.IP == "" {
return "", fmt.Errorf("VM is not running or IP is unavailable")
}
out("Detecting Director port...\n")
var sb strings.Builder
err := s.execSSH(cfg, state,
"sudo kubectl get svc -A -o jsonpath='{.items[*].spec.ports[?(@.port==11717)].nodePort}' 2>&1",
func(line string) {
sb.WriteString(line)
out(line)
},
)
if err != nil {
return "", err
}
port := strings.TrimSpace(sb.String())
if !isNumeric(port) {
return "", fmt.Errorf("could not determine Director port - is the battlegroup running?")
}
return fmt.Sprintf("http://%s:%s/", state.IP, port), nil
}

func isNumeric(s string) bool {
if s == "" {
return false
}
for _, c := range s {
if c < '0' || c > '9' {
return false
}
}
return true
}

// ── update endpoints ───────────────────────────────────────────────────────────

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, VersionResponse{Version: build.Version})
}

func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	cfg := config.Get()
	if cfg.GitHubRepo == "" {
		writeJSON(w, UpdateCheckResponse{Error: "githubRepo not configured"})
		return
	}
	info, err := updater.CheckForUpdate(cfg.GitHubRepo, build.Version)
	if err != nil {
		writeJSON(w, UpdateCheckResponse{Error: err.Error()})
		return
	}
	writeJSON(w, UpdateCheckResponse{
		Current:   info.Current,
		Latest:    info.Latest,
		HasUpdate: info.HasUpdate,
		SvcURL:    info.SvcURL,
		GUIURL:    info.GUIURL,
	})
}

// handleUpdateApply streams the service update over SSE, then self-restarts.
func (s *Server) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, _ := w.(http.Flusher)

	emit := func(typ, line string) {
		ev, _ := json.Marshal(SSEEvent{Type: typ, Line: line})
		fmt.Fprintf(w, "data: %s\n\n", ev)
		if flusher != nil {
			flusher.Flush()
		}
	}

	cfg := config.Get()
	if cfg.GitHubRepo == "" {
		emit("done", "")
		doneEvent, _ := json.Marshal(SSEEvent{Type: "done", Error: "githubRepo not configured"})
		fmt.Fprintf(w, "data: %s\n\n", doneEvent)
		return
	}

	emit("output", "Checking for updates...\n")
	info, err := updater.CheckForUpdate(cfg.GitHubRepo, build.Version)
	if err != nil {
		ev, _ := json.Marshal(SSEEvent{Type: "done", Error: err.Error()})
		fmt.Fprintf(w, "data: %s\n\n", ev)
		return
	}
	if !info.HasUpdate {
		ev, _ := json.Marshal(SSEEvent{Type: "done", Line: ""})
		fmt.Fprintf(w, "data: %s\n\n", ev)
		return
	}
	emit("output", fmt.Sprintf("Update available: %s → %s\n", info.Current, info.Latest))

	// Download service binary.
	if info.SvcURL != "" {
		emit("output", "Downloading service binary...\n")
		tmpSvc, err := updater.DownloadToTemp(info.SvcURL, func(dl, total int64) {
			if total > 0 {
				emit("output", fmt.Sprintf("  %.0f%%\n", float64(dl)/float64(total)*100))
			}
		})
		if err != nil {
			ev, _ := json.Marshal(SSEEvent{Type: "done", Error: "download svc: " + err.Error()})
			fmt.Fprintf(w, "data: %s\n\n", ev)
			return
		}
		emit("output", "Applying service update...\n")
		svcPath, _ := os.Executable()
		if err := updater.ApplyUpdate(tmpSvc, svcPath); err != nil {
			ev, _ := json.Marshal(SSEEvent{Type: "done", Error: "apply svc: " + err.Error()})
			fmt.Fprintf(w, "data: %s\n\n", ev)
			return
		}
		emit("output", "Service binary updated.\n")
	}

	// Return GUI download URL in done.Line so the GUI can self-update.
	emit("output", "Service update complete. Restarting...\n")
	doneEv, _ := json.Marshal(SSEEvent{Type: "done", Line: info.GUIURL})
	fmt.Fprintf(w, "data: %s\n\n", doneEv)
	if flusher != nil {
		flusher.Flush()
	}

	// Restart the service after a short delay so the HTTP response is fully sent.
	go func() {
		time.Sleep(2 * time.Second)
		svcPath, _ := os.Executable()
		cmd := exec.Command(svcPath, "--run")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = cmd.Start()
		os.Exit(0)
	}()
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

