package api

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"github.com/oldbear24/DuneManager/internal/build"
	"github.com/oldbear24/DuneManager/internal/config"
	"github.com/oldbear24/DuneManager/internal/logging"
	"github.com/oldbear24/DuneManager/internal/runner"
	"github.com/oldbear24/DuneManager/internal/updater"
	"github.com/oldbear24/DuneManager/internal/vm"
	"github.com/oldbear24/DuneManager/internal/winsvc"
)

// Server wraps the HTTP service that runs in the background process.
type Server struct {
	httpServer *http.Server
	mu         sync.Mutex
	busy       bool
	activeKill func()
	killable   bool
	killQueued bool
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
		execErr = s.execSSHRotate(out, cfg, state, req.Password)
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
	if !state.Running || state.IP == "" {
		return fmt.Errorf("VM is not running or IP is unavailable")
	}
	return execSSHWithKey(state.IP, cfg.SSHKeyPath, remoteCmd, out)
}

// execSSHWithPassword connects to the VM using password authentication (Go
// native SSH, no subprocess) and streams the remote command output to out.
// Used for initial key installation before any key file exists.
func execSSHWithPassword(ip, password, remoteCmd string, out func(string)) error {
	cfg := &gossh.ClientConfig{
		User:            "dune",
		Auth:            []gossh.AuthMethod{gossh.Password(password)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         15 * time.Second,
	}
	return execGoSSH(ip, cfg, remoteCmd, out)
}

// execSSHWithKey connects to the VM using public key authentication (Go native
// SSH, no subprocess). This avoids subprocess/service-context issues where the
// ssh.exe subprocess can hang waiting for interactive password input.
func execSSHWithKey(ip, keyPath, remoteCmd string, out func(string)) error {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("read SSH key %s: %w", keyPath, err)
	}
	signer, err := gossh.ParsePrivateKey(keyData)
	if err != nil {
		return fmt.Errorf("parse SSH key: %w", err)
	}
	cfg := &gossh.ClientConfig{
		User:            "dune",
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         15 * time.Second,
	}
	logging.Infof("SSH(go): connecting to %s with key %s cmd=%s", ip, keyPath, remoteCmd)
	return execGoSSH(ip, cfg, remoteCmd, out)
}

// execGoSSH is the shared transport for execSSHWithPassword and execSSHWithKey.
func execGoSSH(ip string, cfg *gossh.ClientConfig, remoteCmd string, out func(string)) error {
	client, err := gossh.Dial("tcp", net.JoinHostPort(ip, "22"), cfg)
	if err != nil {
		return fmt.Errorf("SSH connect: %w", err)
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("SSH session: %w", err)
	}
	defer sess.Close()

	pr, pw := io.Pipe()
	sess.Stdout = pw
	sess.Stderr = pw

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			out(scanner.Text() + "\n")
		}
	}()

	runErr := sess.Run(remoteCmd)
	_ = pw.Close()
	wg.Wait()
	return runErr
}

// fixSSHKeyPerms ensures the private key has Windows permissions strict enough
// for OpenSSH to accept it. Uses PowerShell to create a completely fresh ACL
// (disables inheritance, removes all previous ACEs, grants only SYSTEM and
// Administrators full control) so no leftover user ACEs remain.
func fixSSHKeyPerms(keyPath string) {
	if _, err := os.Stat(keyPath); err != nil {
		logging.Warningf("fixSSHKeyPerms: key file not found at %s: %v", keyPath, err)
		return
	}
	script := fmt.Sprintf(`
$p = '%s'
$acl = New-Object System.Security.AccessControl.FileSecurity
$acl.SetAccessRuleProtection($true, $false)
$acl.AddAccessRule([System.Security.AccessControl.FileSystemAccessRule]::new('NT AUTHORITY\SYSTEM', 'FullControl', 'Allow'))
$acl.AddAccessRule([System.Security.AccessControl.FileSystemAccessRule]::new('BUILTIN\Administrators', 'FullControl', 'Allow'))
[System.IO.File]::SetAccessControl($p, $acl)
`, keyPath)
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := cmd.CombinedOutput(); err != nil {
		logging.Errorf("fixSSHKeyPerms: failed for %s: %v — %s", keyPath, err, strings.TrimSpace(string(out)))
	} else {
		logging.Infof("fixSSHKeyPerms: OK for %s", keyPath)
	}
}

func (s *Server) execBattlegroup(cfg config.File, state *vm.State, subcommand string, out func(string)) error {
	logging.Infof("battlegroup: cmd=%s vmRunning=%v ip=%s keyPath=%s", subcommand, state.Running, state.IP, cfg.SSHKeyPath)
	remoteCmd := fmt.Sprintf("TERM=dumb /home/dune/.dune/bin/battlegroup %s 2>&1", subcommand)
	hasOutput := false
	err := s.execSSH(cfg, state, remoteCmd, func(line string) {
		stripped := stripANSI(line)
		if strings.TrimSpace(stripped) != "" {
			hasOutput = true
		}
		logging.Infof("battlegroup output: %s", strings.TrimRight(stripped, "\n"))
		out(stripped)
	})
	logging.Infof("battlegroup: cmd=%s done err=%v hasOutput=%v", subcommand, err, hasOutput)
	if err == nil && !hasOutput {
		out(fmt.Sprintf("(battlegroup %s returned no output)\n", subcommand))
	}
	return err
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

func (s *Server) execSSHRotate(out func(string), cfg config.File, state *vm.State, vmPassword string) error {
	if !state.Running || state.IP == "" {
		return fmt.Errorf("VM is not running or IP is unavailable")
	}

	// Use the key path from config — the service may run as a different user
	// (LocalSystem) so LOCALAPPDATA would resolve to the system profile, not
	// the user profile where the GUI stored the config and expects the key.
	keyPath := cfg.SSHKeyPath
	keyDir := filepath.Dir(keyPath)

	// Use os.CreateTemp to get a unique temp path, then remove it so ssh-keygen
	// can create the file itself.
	tmpFile, err := os.CreateTemp("", "dune-newkey-")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tempStem := tmpFile.Name()
	tmpFile.Close()
	_ = os.Remove(tempStem)
	defer func() {
		_ = os.Remove(tempStem)
		_ = os.Remove(tempStem + ".pub")
	}()

	out("Generating new SSH key pair...\n")
	hostname, _ := os.Hostname()
	keygen := exec.Command("ssh-keygen", "-t", "ed25519", "-f", tempStem, "-N", "", "-q",
		"-C", "vm-server@"+hostname)
	keygen.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if output, err := keygen.CombinedOutput(); err != nil {
		return fmt.Errorf("ssh-keygen failed: %w — %s", err, strings.TrimSpace(string(output)))
	}

	pubData, err := os.ReadFile(tempStem + ".pub")
	if err != nil {
		return fmt.Errorf("read public key: %w", err)
	}
	b64Pub := base64.StdEncoding.EncodeToString(pubData)

	// Build a self-contained shell script, then base64-encode the whole thing
	// so it can be piped through a single SSH command without quoting issues.
	remoteScript := "#!/bin/sh\n" +
		"set -e\n" +
		"mkdir -p $HOME/.ssh\n" +
		"chmod 700 $HOME/.ssh\n" +
		"echo " + b64Pub + " | base64 -d > $HOME/.ssh/authorized_keys.new\n" +
		"chmod 600 $HOME/.ssh/authorized_keys.new\n" +
		"mv $HOME/.ssh/authorized_keys.new $HOME/.ssh/authorized_keys\n" +
		"echo ROTATE_OK\n"
	b64Script := base64.StdEncoding.EncodeToString([]byte(remoteScript))
	installCmd := "echo " + b64Script + " | base64 -d | sh"

	out("Installing new public key on VM...\n")
	// Try key auth first (normal case). If it fails and a password was
	// supplied, fall back to password auth (handles key-out-of-sync scenarios).
	installErr := s.execSSH(cfg, state, installCmd, out)
	if installErr != nil {
		if vmPassword == "" {
			return fmt.Errorf("install public key: %w — provide the VM password in the dialog to recover", installErr)
		}
		out("Key auth failed — retrying with password authentication.\n")
		if err := execSSHWithPassword(state.IP, vmPassword, installCmd, out); err != nil {
			return fmt.Errorf("install public key (password auth): %w", err)
		}
	}

	out("Verifying new key authenticates...\n")
	if err := execSSHWithKey(state.IP, tempStem, "true", func(string) {}); err != nil {
		return fmt.Errorf("new key verification failed — key was installed but does not authenticate: %w", err)
	}

	// Replace local key files.
	if err := os.MkdirAll(keyDir, 0755); err != nil {
		return fmt.Errorf("create key directory: %w", err)
	}
	_ = os.Remove(keyPath)
	_ = os.Remove(keyPath + ".pub")
	if err := renameOrCopy(tempStem, keyPath, 0600); err != nil {
		return fmt.Errorf("save new private key: %w", err)
	}
	if err := renameOrCopy(tempStem+".pub", keyPath+".pub", 0644); err != nil {
		return fmt.Errorf("save new public key: %w", err)
	}

	// Apply strict ACL on the new private key so OpenSSH accepts it.
	fixSSHKeyPerms(keyPath)

	out(fmt.Sprintf("New SSH key installed at:\n  %s\n", keyPath))
	return nil
}

func (s *Server) execPasswordChange(out func(string), cfg config.File, state *vm.State, password string) error {
	if !state.Running || state.IP == "" {
		return fmt.Errorf("VM is not running or IP is unavailable")
	}
	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}

	// Base64-encode "dune:<password>\n" so special characters in the password
	// are safely passed through the shell pipeline.
	b64 := base64.StdEncoding.EncodeToString([]byte("dune:" + password + "\n"))
	remoteCmd := fmt.Sprintf("echo %s | base64 -d | sudo -n chpasswd && echo PWOK", b64)

	var gotPWOK bool
	err := s.execSSH(cfg, state, remoteCmd, func(line string) {
		if strings.Contains(line, "PWOK") {
			gotPWOK = true
			return // don't echo the sentinel to the UI
		}
		out(line)
	})
	if err != nil {
		return fmt.Errorf("password change failed: %w", err)
	}
	if !gotPWOK {
		return fmt.Errorf("password change failed — user may lack passwordless sudo for chpasswd")
	}
	out("Password changed successfully.\n")
	return nil
}

// fileExists returns true if path exists on disk.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// renameOrCopy moves src to dst, falling back to a copy if os.Rename fails
// (e.g. cross-device). perm is used only for the copy fallback.
func renameOrCopy(src, dst string, perm os.FileMode) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, data, perm); err != nil {
		return err
	}
	_ = os.Remove(src)
	return nil
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

var ansiEscapeRE = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b[^[a-zA-Z]*[a-zA-Z]|\x0f|\x0e`)

// stripANSI removes ANSI terminal escape sequences from a string.
func stripANSI(s string) string {
	return ansiEscapeRE.ReplaceAllString(s, "")
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
			WaitPID:          os.Getpid(),
			SourcePath:       tmpSvc,
			TargetPath:       svcPath,
			StartServiceName: winsvc.ServiceName,
			HideWindow:       true,
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

// handleServiceRestart responds immediately then restarts the service via the Windows SCM.
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

	if err := updater.LaunchHelper(updater.HelperPlan{
		WaitPID:          os.Getpid(),
		StartServiceName: winsvc.ServiceName,
		HideWindow:       true,
	}); err != nil {
		logging.Errorf("service restart handoff failed: %v", err)
		http.Error(w, "restart failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "restarting"})
	go func() {
		defer s.endExclusive()
		time.Sleep(500 * time.Millisecond)
		logging.Infof("service exiting for SCM restart")
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
