package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/oldbear24/DuneManager/internal/build"
	"github.com/oldbear24/DuneManager/internal/config"
	"github.com/oldbear24/DuneManager/internal/logging"
	"github.com/oldbear24/DuneManager/internal/runner"
	"github.com/oldbear24/DuneManager/internal/updater"
	"github.com/oldbear24/DuneManager/internal/vm"
)

// Server wraps the HTTP service that runs in the background process.
type Server struct {
	httpServer  *http.Server
	restartArgs []string
	mu          sync.Mutex
	busy        bool
	activeKill  func()
	killable    bool
	killQueued  bool
}

// NewServer builds the HTTP mux and server struct but does not start listening.
func NewServer(restartArgs ...string) *Server {
	if len(restartArgs) == 0 {
		restartArgs = []string{"--run"}
	}
	s := &Server{restartArgs: append([]string(nil), restartArgs...)}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/exec", s.handleExec)
	mux.HandleFunc("/api/kill", s.handleKill)
	mux.HandleFunc("/api/version", s.handleVersion)
	mux.HandleFunc("/api/update/check", s.handleUpdateCheck)
	mux.HandleFunc("/api/update/apply", s.handleUpdateApply)
	mux.HandleFunc("/api/service/restart", s.handleServiceRestart)
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
		logging.Warningf("status lookup failed for VM %q: %v", cfg.VMName, err)
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
		logging.Warningf("rejected exec request with method %s", r.Method)
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var req ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logging.Warningf("rejected exec request with invalid JSON: %v", err)
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	startedAt := time.Now()
	logging.Infof("exec request started: cmd=%s", req.Cmd)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flush := func() {
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	if !s.beginExclusive(true) {
		logging.Warningf("exec request rejected while busy: cmd=%s", req.Cmd)
		writeSSE(w, SSEEvent{Type: "done", Error: "busy: another command is running"})
		flush()
		return
	}

	defer func() {
		s.endExclusive()
	}()

	// If the client disconnects, kill the active process.
	ctx := r.Context()
	abortDone := make(chan struct{})
	go func() {
		defer close(abortDone)
		select {
		case <-ctx.Done():
			logging.Warningf("exec request aborted by client: cmd=%s", req.Cmd)
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
		execErr = s.execBattlegroup(cfg, state, "status", out)
	case "bg-start":
		execErr = s.execBattlegroup(cfg, state, "start", out)
	case "bg-stop":
		execErr = s.execBattlegroup(cfg, state, "stop", out)
	case "bg-restart":
		execErr = s.execBattlegroup(cfg, state, "restart", out)
	case "bg-update":
		execErr = s.execBattlegroup(cfg, state, "update", out)
	case "bg-backup":
		execErr = s.execBattlegroup(cfg, state, "backup", out)
	case "bg-swap":
		execErr = s.execBattlegroup(cfg, state, "enable-experimental-swap", out)
	case "director-port":
		result, execErr = s.execDirectorPort(out, cfg, state)
	default:
		execErr = fmt.Errorf("unknown command: %s", req.Cmd)
	}

	if execErr != nil {
		logging.Errorf("exec request failed: cmd=%s duration=%s err=%v", req.Cmd, time.Since(startedAt).Round(time.Millisecond), execErr)
		writeSSE(w, SSEEvent{Type: "done", Error: execErr.Error()})
	} else {
		logging.Infof("exec request finished: cmd=%s duration=%s", req.Cmd, time.Since(startedAt).Round(time.Millisecond))
		writeSSE(w, SSEEvent{Type: "done", Line: result})
	}
	flush()
}

// handleKill forcibly terminates the active command process.
func (s *Server) handleKill(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	k := s.activeKill
	canQueue := s.busy && s.killable && k == nil
	if canQueue {
		s.killQueued = true
	}
	s.mu.Unlock()
	if k != nil {
		logging.Warningf("kill requested for active command")
		k()
	} else if canQueue {
		logging.Warningf("kill queued for starting command")
	} else {
		logging.Infof("kill requested with no active command")
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeSSE(w http.ResponseWriter, evt SSEEvent) {
	data, _ := json.Marshal(evt)
	fmt.Fprintf(w, "data: %s\n\n", data)
}

// execCmd starts args, registers the kill func, and blocks until done.
func (s *Server) execCmd(args []string, out func(string)) error {
	return s.execCmdWithOptions(args, out, true)
}

func (s *Server) execCmdWithOptions(args []string, out func(string), closeStdin bool) error {
	done := make(chan error, 1)
	stdin, kill, err := runner.RunInteractive(args, out, func(e error) { done <- e })
	if err != nil {
		return err
	}
	if closeStdin {
		_ = stdin.Close()
	}
	queuedKill := false
	s.mu.Lock()
	s.activeKill = kill
	queuedKill = s.killQueued
	s.mu.Unlock()
	if queuedKill {
		kill()
	}
	return <-done
}

func (s *Server) execPS(script string, out func(string)) error {
	return s.execCmd(
		[]string{"powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script},
		out,
	)
}

func (s *Server) execSSH(cfg config.File, state *vm.State, remoteCmd string, out func(string)) error {
	return s.execSSHWithTTY(cfg, state, remoteCmd, false, out)
}

func (s *Server) execSSHWithTTY(cfg config.File, state *vm.State, remoteCmd string, forceTTY bool, out func(string)) error {
	if !state.Running || state.IP == "" {
		return fmt.Errorf("VM is not running or IP is unavailable")
	}
	args := []string{"ssh"}
	if forceTTY {
		args = append(args, "-tt")
	}
	args = append(args,
		"-o", "StrictHostKeyChecking=no",
		"-o", "LogLevel=QUIET",
		"-i", cfg.SSHKeyPath,
		fmt.Sprintf("dune@%s", state.IP),
		remoteCmd,
	)
	return s.execCmdWithOptions(args, out, !forceTTY)
}

func (s *Server) execBattlegroup(cfg config.File, state *vm.State, subcommand string, out func(string)) error {
	return s.execSSHWithTTY(cfg, state, "/home/dune/.dune/bin/battlegroup "+subcommand, true, out)
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
		logging.Warningf("update check skipped: githubRepo not configured")
		writeJSON(w, UpdateCheckResponse{Error: "githubRepo not configured"})
		return
	}
	info, err := updater.CheckForUpdate(cfg.GitHubRepo, build.Version)
	if err != nil {
		logging.Errorf("update check failed for %s: %v", cfg.GitHubRepo, err)
		writeJSON(w, UpdateCheckResponse{Error: err.Error()})
		return
	}
	logging.Infof("update check completed: current=%s latest=%s hasUpdate=%t", info.Current, info.Latest, info.HasUpdate)
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
	logging.Infof("service update requested")
	if !s.beginExclusive(false) {
		logging.Warningf("service update rejected while busy")
		http.Error(w, "busy: another command is running", http.StatusConflict)
		return
	}
	release := true
	defer func() {
		if release {
			s.endExclusive()
		}
	}()

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
		logging.Warningf("service update skipped: githubRepo not configured")
		emit("done", "")
		doneEvent, _ := json.Marshal(SSEEvent{Type: "done", Error: "githubRepo not configured"})
		fmt.Fprintf(w, "data: %s\n\n", doneEvent)
		return
	}

	emit("output", "Checking for updates...\n")
	info, err := updater.CheckForUpdate(cfg.GitHubRepo, build.Version)
	if err != nil {
		logging.Errorf("service update check failed for %s: %v", cfg.GitHubRepo, err)
		ev, _ := json.Marshal(SSEEvent{Type: "done", Error: err.Error()})
		fmt.Fprintf(w, "data: %s\n\n", ev)
		return
	}
	if !info.HasUpdate {
		logging.Infof("service update skipped: already on latest version %s", info.Current)
		ev, _ := json.Marshal(SSEEvent{Type: "done", Line: ""})
		fmt.Fprintf(w, "data: %s\n\n", ev)
		return
	}
	logging.Infof("service update available: %s -> %s", info.Current, info.Latest)
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
			logging.Errorf("service update download failed: %v", err)
			ev, _ := json.Marshal(SSEEvent{Type: "done", Error: "download svc: " + err.Error()})
			fmt.Fprintf(w, "data: %s\n\n", ev)
			return
		}
		emit("output", "Applying service update...\n")
		svcPath, _ := os.Executable()
		if err := updater.LaunchHelper(updater.HelperPlan{
			WaitPID:     os.Getpid(),
			SourcePath:  tmpSvc,
			TargetPath:  svcPath,
			RestartPath: svcPath,
			RestartArgs: s.restartArgs,
			HideWindow:  true,
		}); err != nil {
			logging.Errorf("service update apply failed: %v", err)
			ev, _ := json.Marshal(SSEEvent{Type: "done", Error: "apply svc: " + err.Error()})
			fmt.Fprintf(w, "data: %s\n\n", ev)
			return
		}
		logging.Infof("service update handed off to updater helper")
		emit("output", "Service update staged.\n")
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
		defer s.endExclusive()
		time.Sleep(2 * time.Second)
		logging.Infof("service exiting for updater helper handoff")
		os.Exit(0)
	}()
	release = false
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// handleServiceRestart responds immediately then restarts the service process.
func (s *Server) handleServiceRestart(w http.ResponseWriter, r *http.Request) {
	logging.Infof("service restart requested")
	if !s.beginExclusive(false) {
		logging.Warningf("service restart rejected while busy")
		http.Error(w, "busy: another command is running", http.StatusConflict)
		return
	}
	release := true
	defer func() {
		if release {
			s.endExclusive()
		}
	}()

	svcPath, err := os.Executable()
	if err != nil {
		logging.Errorf("service restart failed to resolve executable path: %v", err)
		http.Error(w, "restart failed: executable path unavailable", http.StatusInternalServerError)
		return
	}
	if err := updater.LaunchHelper(updater.HelperPlan{
		WaitPID:     os.Getpid(),
		RestartPath: svcPath,
		RestartArgs: s.restartArgs,
		HideWindow:  true,
	}); err != nil {
		logging.Errorf("service restart handoff failed: %v", err)
		http.Error(w, "restart failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "restarting"})
	go func() {
		defer s.endExclusive()
		time.Sleep(500 * time.Millisecond)
		logging.Infof("service exiting for restart helper handoff")
		os.Exit(0)
	}()
	release = false
}

func (s *Server) beginExclusive(killable bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy {
		return false
	}
	s.busy = true
	s.activeKill = nil
	s.killable = killable
	s.killQueued = false
	return true
}

func (s *Server) endExclusive() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.busy = false
	s.activeKill = nil
	s.killable = false
	s.killQueued = false
}
