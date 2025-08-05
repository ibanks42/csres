package main

import (
	"fmt"
	"log"
	"syscall"
	"unsafe"
)

// DEVMODE represents the Win32 DEVMODE structure
type DEVMODE struct {
	DeviceName       [32]uint16
	SpecVersion      uint16
	DriverVersion    uint16
	Size             uint16
	DriverExtra      uint16
	Fields           uint32
	X                int32
	Y                int32
	Orientation      uint32
	FixedOutput      uint32
	Color            int16
	Duplex           int16
	YResolution      int16
	TTOption         int16
	Collate          int16
	FormName         [32]uint16
	LogPixels        uint16
	BitsPerPel       uint32
	PelsWidth        uint32
	PelsHeight       uint32
	DisplayFlags     uint32
	DisplayFrequency uint32
	ICMMethod        uint32
	ICMIntent        uint32
	MediaType        uint32
	DitherType       uint32
	Reserved1        uint32
	Reserved2        uint32
	PanningWidth     uint32
	PanningHeight    uint32
}

// DISPLAY_DEVICE represents the Win32 DISPLAY_DEVICE structure
type DISPLAY_DEVICE struct {
	Cb           uint32
	DeviceName   [32]uint16
	DeviceString [128]uint16
	StateFlags   uint32
	DeviceID     [128]uint16
	DeviceKey    [128]uint16
}

const (
	ENUM_CURRENT_SETTINGS  = 0xFFFFFFFF
	ENUM_REGISTRY_SETTINGS = 0xFFFFFFFE

	// Display device state flags
	DISPLAY_DEVICE_ATTACHED_TO_DESKTOP = 0x00000001
	DISPLAY_DEVICE_PRIMARY_DEVICE      = 0x00000004
	DISPLAY_DEVICE_ACTIVE              = 0x00000001
)

// MonitorInfo represents information about a monitor
type MonitorInfo struct {
	DeviceName   string
	DeviceString string
	IsPrimary    bool
}

// DisplayManager manages display settings
type DisplayManager struct {
	user32                       *syscall.DLL
	procEnumDisplayDevicesW      *syscall.Proc
	procEnumDisplaySettingsW     *syscall.Proc
	procChangeDisplaySettingsExW *syscall.Proc
}

// NewDisplayManager creates a new DisplayManager instance
func NewDisplayManager() *DisplayManager {
	user32 := syscall.MustLoadDLL("user32.dll")
	return &DisplayManager{
		user32:                       user32,
		procEnumDisplayDevicesW:      user32.MustFindProc("EnumDisplayDevicesW"),
		procEnumDisplaySettingsW:     user32.MustFindProc("EnumDisplaySettingsW"),
		procChangeDisplaySettingsExW: user32.MustFindProc("ChangeDisplaySettingsExW"),
	}
}

// GetAvailableMonitors returns a list of available monitors
func (dm *DisplayManager) GetAvailableMonitors() ([]MonitorInfo, error) {
	var monitors []MonitorInfo
	var displayDevice DISPLAY_DEVICE
	displayDevice.Cb = uint32(unsafe.Sizeof(displayDevice))

	for i := uint32(0); ; i++ {
		ret, _, err := dm.procEnumDisplayDevicesW.Call(
			uintptr(unsafe.Pointer(nil)),
			uintptr(i),
			uintptr(unsafe.Pointer(&displayDevice)),
			uintptr(0),
		)

		if err != nil && err != syscall.Errno(0) {
			return nil, fmt.Errorf("failed to enumerate display devices: %w", err)
		}

		if ret == 0 {
			break // No more devices
		}

		deviceName := syscall.UTF16ToString(displayDevice.DeviceName[:])
		deviceString := syscall.UTF16ToString(displayDevice.DeviceString[:])

		// Log the device state for debugging
		log.Printf("Found display device: Name=%s String=%s Flags=0x%x", deviceName, deviceString, displayDevice.StateFlags)

		// Include monitors that are either attached to desktop or active
		if displayDevice.StateFlags&(DISPLAY_DEVICE_ATTACHED_TO_DESKTOP|DISPLAY_DEVICE_ACTIVE) != 0 {
			monitor := MonitorInfo{
				DeviceName:   deviceName,
				DeviceString: deviceString,
				IsPrimary:    displayDevice.StateFlags&DISPLAY_DEVICE_PRIMARY_DEVICE != 0,
			}

			monitors = append(monitors, monitor)
		}
	}

	return monitors, nil
}

// GetCurrentResolution retrieves the current display resolution for primary monitor
func (dm *DisplayManager) GetCurrentResolution() (*Resolution, error) {
	return dm.GetCurrentResolutionForMonitor("")
}

// GetCurrentResolutionForMonitor retrieves the current display resolution for a specific monitor
func (dm *DisplayManager) GetCurrentResolutionForMonitor(monitorName string) (*Resolution, error) {
	var devMode DEVMODE
	devMode.Size = uint16(unsafe.Sizeof(devMode))

	// Convert monitorName to UTF16 pointer
	var monitorNamePtr *uint16
	if monitorName != "" {
		monitorNameUtf16, err := syscall.UTF16PtrFromString(monitorName)
		if err != nil {
			return nil, fmt.Errorf("failed to convert monitor name to UTF16: %w", err)
		}
		monitorNamePtr = monitorNameUtf16
	}

	ret, _, err := dm.procEnumDisplaySettingsW.Call(
		uintptr(unsafe.Pointer(monitorNamePtr)),
		uintptr(ENUM_CURRENT_SETTINGS),
		uintptr(unsafe.Pointer(&devMode)),
	)

	if ret == 0 {
		if err != nil {
			return nil, fmt.Errorf("failed to get display settings: %w", err)
		}
		return nil, fmt.Errorf("failed to get display settings")
	}

	return &Resolution{
		Width:     uint32(devMode.PelsWidth),
		Height:    uint32(devMode.PelsHeight),
		Frequency: uint32(devMode.DisplayFrequency),
	}, nil
}

// GetAvailableResolutions returns a list of available resolutions for a monitor
func (dm *DisplayManager) GetAvailableResolutions(monitorName string) ([]Resolution, error) {
	var resolutions []Resolution
	var devMode DEVMODE
	devMode.Size = uint16(unsafe.Sizeof(devMode))

	// Convert monitorName to UTF16 pointer
	var monitorNamePtr *uint16
	if monitorName != "" {
		monitorNameUtf16, err := syscall.UTF16PtrFromString(monitorName)
		if err != nil {
			return nil, fmt.Errorf("failed to convert monitor name to UTF16: %w", err)
		}
		monitorNamePtr = monitorNameUtf16
	}

	// Enumerate all display settings
	for modeNum := uint32(0); ; modeNum++ {
		ret, _, _ := dm.procEnumDisplaySettingsW.Call(
			uintptr(unsafe.Pointer(monitorNamePtr)),
			uintptr(modeNum),
			uintptr(unsafe.Pointer(&devMode)),
		)

		if ret == 0 {
			break // No more modes
		}

		resolution := Resolution{
			Width:     uint32(devMode.PelsWidth),
			Height:    uint32(devMode.PelsHeight),
			Frequency: uint32(devMode.DisplayFrequency),
		}

		// Check if this resolution is already in the list
		isDuplicate := false
		for _, r := range resolutions {
			if r.Width == resolution.Width && r.Height == resolution.Height && r.Frequency == resolution.Frequency {
				isDuplicate = true
				break
			}
		}

		if !isDuplicate {
			resolutions = append(resolutions, resolution)
		}
	}

	return resolutions, nil
}

// SetResolution changes the display resolution for a specific monitor
func (dm *DisplayManager) SetResolution(monitorName string, resolution Resolution) error {
	var devMode DEVMODE
	devMode.Size = uint16(unsafe.Sizeof(devMode))
	devMode.Fields = 0x00180000 // DM_PELSWIDTH | DM_PELSHEIGHT | DM_DISPLAYFREQUENCY
	devMode.PelsWidth = uint32(resolution.Width)
	devMode.PelsHeight = uint32(resolution.Height)
	devMode.DisplayFrequency = uint32(resolution.Frequency)

	// Convert monitorName to UTF16 pointer
	var monitorNamePtr *uint16
	if monitorName != "" {
		monitorNameUtf16, err := syscall.UTF16PtrFromString(monitorName)
		if err != nil {
			return fmt.Errorf("failed to convert monitor name to UTF16: %w", err)
		}
		monitorNamePtr = monitorNameUtf16
	}

	// Try to change the display settings
	const maxRetries = 3
	var lastError error

	for i := 0; i < maxRetries; i++ {
		ret, _, err := dm.procChangeDisplaySettingsExW.Call(
			uintptr(unsafe.Pointer(monitorNamePtr)),
			uintptr(unsafe.Pointer(&devMode)),
			0,
			0,
			0,
		)

		if ret == 0 {
			return nil // Success
		}

		lastError = err
		log.Printf("Attempt %d to change resolution failed: %v", i+1, err)
	}

	return fmt.Errorf("failed to change resolution after %d attempts. Last error: %v", maxRetries, lastError)
}

// IsResolutionEqual compares two resolutions for equality
func IsResolutionEqual(r1, r2 Resolution) bool {
	return r1.Width == r2.Width && r1.Height == r2.Height && r1.Frequency == r2.Frequency
}
