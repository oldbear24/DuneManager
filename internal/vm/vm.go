package vm

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

// State describes the current status of the Dune VM.
type State struct {
	Exists  bool
	Running bool
	VMState string // "Running", "Off", "Saved", "Paused", "missing"
	IP      string
}

// GetState queries Hyper-V for the VM's current state and IP address.
func GetState(vmName string) (*State, error) {
	script := fmt.Sprintf(`
$vm = Get-VM -Name '%s' -ErrorAction SilentlyContinue
if ($null -eq $vm) {
    [pscustomobject]@{ exists=$false; state='missing'; ip='' } | ConvertTo-Json -Compress
} else {
    $ip = ''
    if ($vm.State -eq 'Running') {
        $addr = (Get-VMNetworkAdapter -VMName '%s').IPAddresses |
                Where-Object { $_ -match '^\d+\.\d+\.\d+\.\d+$' } |
                Select-Object -First 1
        if ($addr) { $ip = $addr }
    }
    [pscustomobject]@{ exists=$true; state=$vm.State.ToString(); ip=$ip } | ConvertTo-Json -Compress
}`, vmName, vmName)

	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return &State{VMState: "error"}, fmt.Errorf("hyper-v query failed: %w", err)
	}

	line := strings.TrimSpace(string(out))
	var raw struct {
		Exists bool   `json:"exists"`
		State  string `json:"state"`
		IP     string `json:"ip"`
	}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return &State{VMState: "error"}, fmt.Errorf("parse error (%s): %w", line, err)
	}

	return &State{
		Exists:  raw.Exists,
		Running: raw.State == "Running",
		VMState: raw.State,
		IP:      raw.IP,
	}, nil
}
