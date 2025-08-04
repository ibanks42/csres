package main

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

const (
	TH32CS_SNAPPROCESS   = 0x00000002
	INVALID_HANDLE_VALUE = ^uintptr(0)
)

// PROCESSENTRY32 represents an entry in the system's process list
type PROCESSENTRY32 struct {
	DwSize              uint32
	CntUsage            uint32
	Th32ProcessID       uint32
	Th32DefaultHeapID   uintptr
	Th32ModuleID        uint32
	CntThreads          uint32
	Th32ParentProcessID uint32
	PcPriClassBase      int32
	DwFlags             uint32
	SzExeFile           [260]uint16 // MAX_PATH
}

// ProcessMonitor handles process monitoring functionality
type ProcessMonitor struct {
	kernel32dll                  *syscall.LazyDLL
	procCreateToolhelp32Snapshot *syscall.LazyProc
	procProcess32FirstW          *syscall.LazyProc
	procProcess32NextW           *syscall.LazyProc
	procCloseHandle              *syscall.LazyProc
}

// NewProcessMonitor creates a new ProcessMonitor instance
func NewProcessMonitor() *ProcessMonitor {
	kernel32dll := syscall.NewLazyDLL("kernel32.dll")
	return &ProcessMonitor{
		kernel32dll:                  kernel32dll,
		procCreateToolhelp32Snapshot: kernel32dll.NewProc("CreateToolhelp32Snapshot"),
		procProcess32FirstW:          kernel32dll.NewProc("Process32FirstW"),
		procProcess32NextW:           kernel32dll.NewProc("Process32NextW"),
		procCloseHandle:              kernel32dll.NewProc("CloseHandle"),
	}
}

// IsProcessRunning checks if a process with the given name is currently running
func (pm *ProcessMonitor) IsProcessRunning(processName string) (bool, error) {
	processes, err := pm.GetRunningProcesses()
	if err != nil {
		return false, fmt.Errorf("failed to get running processes: %w", err)
	}

	processNameLower := strings.ToLower(processName)
	for _, proc := range processes {
		if strings.ToLower(proc) == processNameLower {
			return true, nil
		}
	}

	return false, nil
}

// GetRunningProcesses returns a list of all currently running process names
func (pm *ProcessMonitor) GetRunningProcesses() ([]string, error) {
	snapshot, _, _ := pm.procCreateToolhelp32Snapshot.Call(
		uintptr(TH32CS_SNAPPROCESS),
		uintptr(0),
	)

	if snapshot == INVALID_HANDLE_VALUE {
		return nil, fmt.Errorf("failed to create process snapshot")
	}
	defer pm.procCloseHandle.Call(snapshot)

	var processes []string
	var pe32 PROCESSENTRY32
	pe32.DwSize = uint32(unsafe.Sizeof(pe32))

	// Get first process
	ret, _, _ := pm.procProcess32FirstW.Call(snapshot, uintptr(unsafe.Pointer(&pe32)))
	if ret == 0 {
		return nil, fmt.Errorf("failed to get first process")
	}

	for {
		// Convert UTF-16 to string
		processName := syscall.UTF16ToString(pe32.SzExeFile[:])
		if processName != "" {
			processes = append(processes, processName)
		}

		// Get next process
		ret, _, _ := pm.procProcess32NextW.Call(snapshot, uintptr(unsafe.Pointer(&pe32)))
		if ret == 0 {
			break // No more processes
		}
	}

	return processes, nil
}

// MonitorProcesses checks which configured applications are currently running
func (pm *ProcessMonitor) MonitorProcesses(config *Config) (map[string]AppConfig, error) {
	runningApps := make(map[string]AppConfig)

	for _, app := range config.Applications {
		isRunning, err := pm.IsProcessRunning(app.ProcessName)
		if err != nil {
			return nil, fmt.Errorf("failed to check if process %s is running: %w", app.ProcessName, err)
		}

		if isRunning {
			runningApps[app.ProcessName] = app
		}
	}

	return runningApps, nil
}
