package winsvc

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const ServiceName = "DuneManager"

// StartElevated starts the DuneManager service by launching an elevated sc.exe
// via ShellExecuteW (runas), which triggers a UAC prompt if needed.
func StartElevated() error {
	shell32 := syscall.NewLazyDLL("shell32.dll")
	shellExecuteW := shell32.NewProc("ShellExecuteW")

	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString("sc.exe")
	args, _ := syscall.UTF16PtrFromString("start " + ServiceName)

	ret, _, _ := shellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(args)),
		0,
		1, // SW_NORMAL
	)
	if ret <= 32 {
		return fmt.Errorf("failed to launch elevated sc.exe (code %d)", ret)
	}
	return nil
}

// Start starts the DuneManager Windows service via the SCM.
func Start() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(ServiceName)
	if err != nil {
		return fmt.Errorf("service %q not found", ServiceName)
	}
	defer s.Close()
	return s.Start()
}

// Stop sends a stop control to the DuneManager Windows service via the SCM.
func Stop() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(ServiceName)
	if err != nil {
		return fmt.Errorf("service %q not found", ServiceName)
	}
	defer s.Close()
	_, err = s.Control(svc.Stop)
	return err
}
