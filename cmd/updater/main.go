package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/oldbear24/DuneManager/internal/updater"
	"golang.org/x/sys/windows"
)

func main() {
	planPath := flag.String("plan", "", "Path to updater plan JSON")
	flag.Parse()

	if *planPath == "" {
		fmt.Fprintln(os.Stderr, "usage: dune-manager-updater.exe --plan <path>")
		os.Exit(1)
	}
	if err := run(*planPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(planPath string) error {
	data, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("read plan: %w", err)
	}
	defer os.Remove(planPath)

	var plan updater.HelperPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return fmt.Errorf("parse plan: %w", err)
	}

	if plan.WaitPID > 0 {
		if err := waitForProcessExit(plan.WaitPID, 2*time.Minute); err != nil {
			return err
		}
	}

	if err := updater.ApplyUpdate(plan.SourcePath, plan.TargetPath); err != nil {
		return err
	}

	if plan.RestartPath != "" {
		cmd := exec.Command(plan.RestartPath, plan.RestartArgs...)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    plan.HideWindow,
			CreationFlags: uint32(windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS),
		}
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("restart target: %w", err)
		}
	}

	return nil
}

func waitForProcessExit(pid int, timeout time.Duration) error {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(handle)

	waitMs := uint32(timeout / time.Millisecond)
	status, err := windows.WaitForSingleObject(handle, waitMs)
	if err != nil {
		return fmt.Errorf("wait for process %d: %w", pid, err)
	}
	if status == uint32(windows.WAIT_TIMEOUT) {
		return fmt.Errorf("timed out waiting for process %d to exit", pid)
	}
	return nil
}
