package runner

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

// RunPS executes an inline PowerShell script, streaming each line to onOutput.
func RunPS(script string, onOutput func(string)) error {
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return runAndStream(cmd, onOutput)
}

// RunSSHCmd runs a remote command over SSH, streaming output to onOutput.
func RunSSHCmd(ip, sshKey, remoteCmd string, onOutput func(string)) error {
	cmd := exec.Command("ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "LogLevel=QUIET",
		"-i", sshKey,
		fmt.Sprintf("dune@%s", ip),
		remoteCmd,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return runAndStream(cmd, onOutput)
}

// OpenTerminalWithScript writes psScript to a temp .ps1 file and opens it in a
// new visible console window. It returns a kill func to forcibly close the
// window and calls onDone (if non-nil) when the process exits naturally.
// asAdmin is accepted for API compatibility but is a no-op when the app itself
// already runs elevated.
func OpenTerminalWithScript(psScript string, asAdmin bool, onDone func()) (func(), error) {
	wrapped := "$ErrorActionPreference = 'Stop'\ntry {\n" +
		psScript +
		"\n} catch {\n" +
		"    Write-Host \"`n[ERROR] $($_.Exception.Message)\" -ForegroundColor Red\n" +
		"    if ($_.ScriptStackTrace) { Write-Host $_.ScriptStackTrace -ForegroundColor DarkRed }\n" +
		"    Write-Host ''\n" +
		"    Read-Host 'Press Enter to close'\n" +
		"    exit 1\n" +
		"}\n"

	tmpFile, err := os.CreateTemp("", "dunemanager-*.ps1")
	if err != nil {
		return nil, err
	}
	tmpPath := tmpFile.Name()
	_, _ = tmpFile.WriteString(wrapped)
	_ = tmpFile.Close()

	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", tmpPath)
	// CREATE_NEW_CONSOLE opens a visible terminal window while keeping the
	// process handle so we can kill it on demand.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000010}

	if err := cmd.Start(); err != nil {
		_ = os.Remove(tmpPath)
		return nil, err
	}

	kill := func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}

	go func() {
		_ = cmd.Wait()
		if onDone != nil {
			onDone()
		}
	}()

	return kill, nil
}

// OpenTerminalSSH opens a new console window with an interactive SSH session.
// If remoteCmd is empty, it opens a plain shell on the VM.
// Returns a kill func and calls onDone when the window closes.
func OpenTerminalSSH(ip, sshKey, remoteCmd string, onDone func()) (func(), error) {
	sshArgs := fmt.Sprintf(
		`ssh -t -o StrictHostKeyChecking=no -o LogLevel=QUIET -i "%s" "dune@%s"`,
		sshKey, ip,
	)
	if remoteCmd != "" {
		sshArgs += " " + remoteCmd
	}

	script := sshArgs + "\nWrite-Host ''\nRead-Host 'Press Enter to close'\n"
	return OpenTerminalWithScript(script, false, onDone)
}

// OpenBrowser opens a URL in the default browser.
func OpenBrowser(url string) {
	_ = exec.Command("cmd", "/c", "start", url).Start()
}

// RunInteractive starts the process given by args, streaming stdout and stderr
// line-by-line to onOutput. It returns a WriteCloser connected to the process
// stdin so the caller can send input lines, and a kill func to forcibly
// terminate the process. onDone is called with the exit error when the process
// finishes (may be nil on clean exit).
func RunInteractive(args []string, onOutput func(string), onDone func(error)) (io.WriteCloser, func(), error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}

	kill := func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}

	var wg sync.WaitGroup
	pipe := func(r io.Reader) {
		defer wg.Done()
		scanStream(r, onOutput)
	}
	wg.Add(2)
	go pipe(stdout)
	go pipe(stderr)
	go func() {
		wg.Wait()
		onDone(cmd.Wait())
	}()
	return stdinPipe, kill, nil
}

func runAndStream(cmd *exec.Cmd, onOutput func(string)) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	var wg sync.WaitGroup

	pipe := func(r io.Reader) {
		defer wg.Done()
		scanStream(r, onOutput)
	}

	wg.Add(2)
	go pipe(stdout)
	go pipe(stderr)
	wg.Wait()

	return cmd.Wait()
}

func scanStream(r io.Reader, onOutput func(string)) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	scanner.Split(scanLinesOrCR)
	for scanner.Scan() {
		onOutput(scanner.Text() + "\n")
	}
}

func scanLinesOrCR(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		if data[i] == '\r' && i+1 < len(data) && data[i+1] == '\n' {
			return i + 2, data[:i], nil
		}
		return i + 1, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}
