package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

const (
	HKEY_CURRENT_USER = 0x80000001
	KEY_SET_VALUE     = 0x0002
	KEY_QUERY_VALUE   = 0x0001
	REG_SZ            = 1
)

// handleWindowsStartup manages the Windows startup registry entry
func (g *GUIApp) handleWindowsStartup(enable bool) error {
	keyPath := "SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Run"
	valueName := "CSResolutionMonitor"

	if enable {
		// Add to startup
		exePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}

		// Convert to absolute path
		absPath, err := filepath.Abs(exePath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}

		return g.setRegistryValue(keyPath, valueName, absPath)
	} else {
		// Remove from startup
		return g.deleteRegistryValue(keyPath, valueName)
	}
}

// setRegistryValue sets a registry value
func (g *GUIApp) setRegistryValue(keyPath, valueName, value string) error {
	advapi32 := syscall.NewLazyDLL("advapi32.dll")
	regOpenKeyEx := advapi32.NewProc("RegOpenKeyExW")
	regSetValueEx := advapi32.NewProc("RegSetValueExW")
	regCloseKey := advapi32.NewProc("RegCloseKey")

	keyPathPtr, err := syscall.UTF16PtrFromString(keyPath)
	if err != nil {
		return err
	}

	valueNamePtr, err := syscall.UTF16PtrFromString(valueName)
	if err != nil {
		return err
	}

	valuePtr, err := syscall.UTF16PtrFromString(value)
	if err != nil {
		return err
	}

	var hKey syscall.Handle
	ret, _, _ := regOpenKeyEx.Call(
		uintptr(HKEY_CURRENT_USER),
		uintptr(unsafe.Pointer(keyPathPtr)),
		0,
		uintptr(KEY_SET_VALUE),
		uintptr(unsafe.Pointer(&hKey)),
	)

	if ret != 0 {
		return fmt.Errorf("failed to open registry key (error code: %d)", ret)
	}
	defer regCloseKey.Call(uintptr(hKey))

	valueBytes := (*[256]byte)(unsafe.Pointer(valuePtr))
	valueLen := len(syscall.UTF16ToString((*[256]uint16)(unsafe.Pointer(valuePtr))[:]))*2 + 2

	ret, _, _ = regSetValueEx.Call(
		uintptr(hKey),
		uintptr(unsafe.Pointer(valueNamePtr)),
		0,
		uintptr(REG_SZ),
		uintptr(unsafe.Pointer(&valueBytes[0])),
		uintptr(valueLen),
	)

	if ret != 0 {
		return fmt.Errorf("failed to set registry value (error code: %d)", ret)
	}

	return nil
}

// deleteRegistryValue deletes a registry value
func (g *GUIApp) deleteRegistryValue(keyPath, valueName string) error {
	advapi32 := syscall.NewLazyDLL("advapi32.dll")
	regOpenKeyEx := advapi32.NewProc("RegOpenKeyExW")
	regDeleteValue := advapi32.NewProc("RegDeleteValueW")
	regCloseKey := advapi32.NewProc("RegCloseKey")

	keyPathPtr, err := syscall.UTF16PtrFromString(keyPath)
	if err != nil {
		return err
	}

	valueNamePtr, err := syscall.UTF16PtrFromString(valueName)
	if err != nil {
		return err
	}

	var hKey syscall.Handle
	ret, _, _ := regOpenKeyEx.Call(
		uintptr(HKEY_CURRENT_USER),
		uintptr(unsafe.Pointer(keyPathPtr)),
		0,
		uintptr(KEY_SET_VALUE),
		uintptr(unsafe.Pointer(&hKey)),
	)

	if ret != 0 {
		// Key might not exist, which is fine if we're trying to remove it
		return nil
	}
	defer regCloseKey.Call(uintptr(hKey))

	_, _, _ = regDeleteValue.Call(
		uintptr(hKey),
		uintptr(unsafe.Pointer(valueNamePtr)),
	)

	// Ignore errors when deleting - the value might not exist
	return nil
}

// isInWindowsStartup checks if the application is currently set to start with Windows
func (g *GUIApp) isInWindowsStartup() bool {
	advapi32 := syscall.NewLazyDLL("advapi32.dll")
	regOpenKeyEx := advapi32.NewProc("RegOpenKeyExW")
	regQueryValueEx := advapi32.NewProc("RegQueryValueExW")
	regCloseKey := advapi32.NewProc("RegCloseKey")

	keyPath := "SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Run"
	valueName := "CSResolutionMonitor"

	keyPathPtr, err := syscall.UTF16PtrFromString(keyPath)
	if err != nil {
		return false
	}

	valueNamePtr, err := syscall.UTF16PtrFromString(valueName)
	if err != nil {
		return false
	}

	var hKey syscall.Handle
	ret, _, _ := regOpenKeyEx.Call(
		uintptr(HKEY_CURRENT_USER),
		uintptr(unsafe.Pointer(keyPathPtr)),
		0,
		uintptr(KEY_QUERY_VALUE),
		uintptr(unsafe.Pointer(&hKey)),
	)

	if ret != 0 {
		return false
	}
	defer regCloseKey.Call(uintptr(hKey))

	var valueType uint32
	var dataSize uint32

	ret, _, _ = regQueryValueEx.Call(
		uintptr(hKey),
		uintptr(unsafe.Pointer(valueNamePtr)),
		0,
		uintptr(unsafe.Pointer(&valueType)),
		0,
		uintptr(unsafe.Pointer(&dataSize)),
	)

	return ret == 0
}
