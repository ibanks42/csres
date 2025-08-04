package main

import (
	"fmt"
	"log"
	"syscall"
	"time"
	"unsafe"
)

const (
	ENUM_CURRENT_SETTINGS   = 0xFFFFFFFF
	DISP_CHANGE_SUCCESSFUL  = 0
	DISP_CHANGE_RESTART     = 1
	DISP_CHANGE_FAILED      = 0xFFFFFFFF
	DISP_CHANGE_BADMODE     = 0xFFFFFFFE
	DISP_CHANGE_NOTUPDATED  = 0xFFFFFFFD
	DISP_CHANGE_BADFLAGS    = 0xFFFFFFFC
	DISP_CHANGE_BADPARAM    = 0xFFFFFFFB
	DISP_CHANGE_BADDUALVIEW = 0xFFFFFFFA

	DM_PELSWIDTH        = 0x00080000
	DM_PELSHEIGHT       = 0x00100000
	DM_DISPLAYFREQUENCY = 0x00400000

	DISPLAY_DEVICE_ATTACHED_TO_DESKTOP = 0x00000001
	DISPLAY_DEVICE_PRIMARY_DEVICE      = 0x00000004
	DISPLAY_DEVICE_ACTIVE              = 0x00000002
)

// DEVMODE represents a display settings structure
type DEVMODE struct {
	DmDeviceName       [32]uint16
	DmSpecVersion      uint16
	DmDriverVersion    uint16
	DmSize             uint16
	DmDriverExtra      uint16
	DmFields           uint32
	DmOrientation      int16
	DmPaperSize        int16
	DmPaperLength      int16
	DmPaperWidth       int16
	DmScale            int16
	DmCopies           int16
	DmDefaultSource    int16
	DmPrintQuality     int16
	DmColor            int16
	DmDuplex           int16
	DmYResolution      int16
	DmTTOption         int16
	DmCollate          int16
	DmFormName         [32]uint16
	DmUnusedPadding    uint16
	DmBitsPerPel       uint32
	DmPelsWidth        uint32
	DmPelsHeight       uint32
	DmDisplayFlags     uint32
	DmDisplayFrequency uint32
	DmICMMethod        uint32
	DmICMIntent        uint32
	DmMediaType        uint32
	DmDitherType       uint32
	DmReserved1        uint32
	DmReserved2        uint32
	DmPanningWidth     uint32
	DmPanningHeight    uint32
}

// DISPLAY_DEVICE represents information about a display device
type DISPLAY_DEVICE struct {
	Cb           uint32
	DeviceName   [32]uint16
	DeviceString [128]uint16
	StateFlags   uint32
	DeviceID     [128]uint16
	DeviceKey    [128]uint16
}

// MonitorInfo represents information about a monitor
type MonitorInfo struct {
	DeviceName   string
	DeviceString string
	IsPrimary    bool
}

// DisplayManager handles display resolution changes
type DisplayManager struct {
	user32dll                    *syscall.LazyDLL
	procEnumDisplaySettingsW     *syscall.LazyProc
	procChangeDisplaySettingsW   *syscall.LazyProc
	procChangeDisplaySettingsExW *syscall.LazyProc
	procEnumDisplayDevicesW      *syscall.LazyProc
}

// NewDisplayManager creates a new DisplayManager instance
func NewDisplayManager() *DisplayManager {
	user32dll := syscall.NewLazyDLL("user32.dll")
	return &DisplayManager{
		user32dll:                    user32dll,
		procEnumDisplaySettingsW:     user32dll.NewProc("EnumDisplaySettingsW"),
		procChangeDisplaySettingsW:   user32dll.NewProc("ChangeDisplaySettingsW"),
		procChangeDisplaySettingsExW: user32dll.NewProc("ChangeDisplaySettingsExW"),
		procEnumDisplayDevicesW:      user32dll.NewProc("EnumDisplayDevicesW"),
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
	devMode := new(DEVMODE)

	var deviceNamePtr uintptr
	if monitorName != "" {
		deviceNameUTF16, err := syscall.UTF16PtrFromString(monitorName)
		if err != nil {
			return nil, fmt.Errorf("failed to convert monitor name to UTF16: %w", err)
		}
		deviceNamePtr = uintptr(unsafe.Pointer(deviceNameUTF16))
	}

	ret, _, _ := dm.procEnumDisplaySettingsW.Call(
		deviceNamePtr,
		uintptr(ENUM_CURRENT_SETTINGS),
		uintptr(unsafe.Pointer(devMode)),
	)

	if ret == 0 {
		if monitorName != "" {
			return nil, fmt.Errorf("failed to get current display settings for monitor %s", monitorName)
		}
		return nil, fmt.Errorf("failed to get current display settings for primary monitor")
	}

	return &Resolution{
		Width:     devMode.DmPelsWidth,
		Height:    devMode.DmPelsHeight,
		Frequency: devMode.DmDisplayFrequency,
	}, nil
}

// IsModeSupported checks if a given resolution mode is supported by the monitor
func (dm *DisplayManager) IsModeSupported(targetResolution Resolution, monitorName string) bool {
	devMode := new(DEVMODE)
	var deviceNamePtr uintptr
	if monitorName != "" {
		deviceNameUTF16, err := syscall.UTF16PtrFromString(monitorName)
		if err != nil {
			return false
		}
		deviceNamePtr = uintptr(unsafe.Pointer(deviceNameUTF16))
	}

	// Enumerate all modes and check if our target resolution exists
	for i := uint32(0); ; i++ {
		ret, _, _ := dm.procEnumDisplaySettingsW.Call(
			deviceNamePtr,
			uintptr(i),
			uintptr(unsafe.Pointer(devMode)),
		)

		if ret == 0 {
			break // No more modes
		}

		// Check if this mode matches our target resolution
		if devMode.DmPelsWidth == targetResolution.Width &&
			devMode.DmPelsHeight == targetResolution.Height &&
			(targetResolution.Frequency == 0 || devMode.DmDisplayFrequency == targetResolution.Frequency) {
			return true
		}
	}

	return false
}

// ChangeResolution changes the display resolution for primary monitor
func (dm *DisplayManager) ChangeResolution(resolution Resolution) error {
	return dm.ChangeResolutionForMonitor(resolution, "")
}

// ChangeResolutionForMonitor changes the display resolution for a specific monitor
func (dm *DisplayManager) ChangeResolutionForMonitor(resolution Resolution, monitorName string) error {
	log.Printf("Attempting to change resolution on %s", monitorName)

	// First verify the mode is supported
	if !dm.IsModeSupported(resolution, monitorName) {
		log.Printf("Resolution %dx%d@%dHz is not supported on %s", resolution.Width, resolution.Height, resolution.Frequency, monitorName)
		return fmt.Errorf("resolution %dx%d@%dHz is not supported by monitor %s",
			resolution.Width, resolution.Height, resolution.Frequency,
			monitorName)
	}

	// Get current monitor state to verify it's active
	var displayDevice DISPLAY_DEVICE
	displayDevice.Cb = uint32(unsafe.Sizeof(displayDevice))

	if monitorName != "" {
		ret, _, _ := dm.procEnumDisplayDevicesW.Call(
			uintptr(0),
			uintptr(0),
			uintptr(unsafe.Pointer(&displayDevice)),
			uintptr(0),
		)

		if ret == 0 {
			return fmt.Errorf("failed to get monitor state for %s", monitorName)
		}

		// Log monitor state for debugging
		log.Printf("Checking monitor state for %s: Flags=0x%x", monitorName, displayDevice.StateFlags)

		// Allow monitors that are either attached to desktop or active
		if displayDevice.StateFlags&(DISPLAY_DEVICE_ATTACHED_TO_DESKTOP|DISPLAY_DEVICE_ACTIVE) == 0 {
			return fmt.Errorf("monitor %s is neither active nor attached (flags=0x%x)", monitorName, displayDevice.StateFlags)
		}
	}

	// Get current settings first
	devMode := new(DEVMODE)

	var deviceNamePtr uintptr
	if monitorName != "" {
		deviceNameUTF16, err := syscall.UTF16PtrFromString(monitorName)
		if err != nil {
			return fmt.Errorf("failed to convert monitor name to UTF16: %w", err)
		}
		deviceNamePtr = uintptr(unsafe.Pointer(deviceNameUTF16))
	}

	ret, _, _ := dm.procEnumDisplaySettingsW.Call(
		deviceNamePtr,
		uintptr(ENUM_CURRENT_SETTINGS),
		uintptr(unsafe.Pointer(devMode)),
	)

	if ret == 0 {
		if monitorName != "" {
			return fmt.Errorf("failed to get current display settings for monitor %s", monitorName)
		}
		return fmt.Errorf("failed to get current display settings for primary monitor")
	}

	// Modify the resolution
	newMode := *devMode
	newMode.DmPelsWidth = resolution.Width
	newMode.DmPelsHeight = resolution.Height

	// Preserve existing DmFields and add our modifications
	newMode.DmFields |= DM_PELSWIDTH | DM_PELSHEIGHT

	// Set frequency if specified
	if resolution.Frequency > 0 {
		newMode.DmDisplayFrequency = resolution.Frequency
		newMode.DmFields |= DM_DISPLAYFREQUENCY
	}

	// Ensure the structure size is set correctly
	newMode.DmSize = uint16(unsafe.Sizeof(newMode))

	monitorDesc := "primary monitor"

	// Apply the changes with retry logic
	maxRetries := 3
	var lastError error

	for retry := 0; retry < maxRetries; retry++ {
		if retry > 0 {
			// Small delay between retries
			time.Sleep(500 * time.Millisecond)
		}

		// Apply the changes
		if monitorName != "" {
			// Use ChangeDisplaySettingsEx for specific monitors
			monitorDesc = fmt.Sprintf("monitor %s", monitorName)
			ret, _, err := dm.procChangeDisplaySettingsExW.Call(
				deviceNamePtr,                     // Device name
				uintptr(unsafe.Pointer(&newMode)), // DEVMODE
				uintptr(0),                        // hwnd (not used)
				uintptr(0),                        // dwflags
				uintptr(0),                        // lParam (not used)
			)

			switch ret {
			case uintptr(DISP_CHANGE_SUCCESSFUL):
				return nil
			case uintptr(DISP_CHANGE_RESTART):
				lastError = fmt.Errorf("restart required to apply resolution changes on %s", monitorDesc)
				continue // retry
			case uintptr(DISP_CHANGE_BADMODE):
				return fmt.Errorf("resolution %dx%d@%dHz is not supported by %s. Try a different refresh rate or resolution", resolution.Width, resolution.Height, resolution.Frequency, monitorDesc)
			case uintptr(DISP_CHANGE_FAILED):
				lastError = fmt.Errorf("failed to change display resolution on %s (insufficient permissions or driver issue)", monitorDesc)
				continue // retry
			case uintptr(DISP_CHANGE_NOTUPDATED):
				lastError = fmt.Errorf("unable to write settings to registry for %s", monitorDesc)
				continue // retry
			case uintptr(DISP_CHANGE_BADFLAGS):
				return fmt.Errorf("invalid flags passed for resolution change on %s", monitorDesc)
			case uintptr(DISP_CHANGE_BADPARAM):
				return fmt.Errorf("invalid parameter passed for resolution change on %s", monitorDesc)
			case uintptr(DISP_CHANGE_BADDUALVIEW):
				return fmt.Errorf("unable to change resolution on %s (DualView system)", monitorDesc)
			default:
				lastError = fmt.Errorf("unknown error occurred while changing resolution on %s (error code: %d, system error: %v)", monitorDesc, ret, err)
				continue // retry
			}
		} else {
			// Use ChangeDisplaySettings for primary monitor (no device name needed)
			ret, _, err := dm.procChangeDisplaySettingsW.Call(
				uintptr(unsafe.Pointer(&newMode)), // DEVMODE
				uintptr(0),                        // dwflags
			)

			switch ret {
			case uintptr(DISP_CHANGE_SUCCESSFUL):
				return nil
			case uintptr(DISP_CHANGE_RESTART):
				lastError = fmt.Errorf("restart required to apply resolution changes on %s", monitorDesc)
				continue // retry
			case uintptr(DISP_CHANGE_BADMODE):
				return fmt.Errorf("resolution %dx%d@%dHz is not supported by %s. Try a different refresh rate or resolution", resolution.Width, resolution.Height, resolution.Frequency, monitorDesc)
			case uintptr(DISP_CHANGE_FAILED):
				lastError = fmt.Errorf("failed to change display resolution on %s (insufficient permissions or driver issue)", monitorDesc)
				continue // retry
			case uintptr(DISP_CHANGE_NOTUPDATED):
				lastError = fmt.Errorf("unable to write settings to registry for %s", monitorDesc)
				continue // retry
			case uintptr(DISP_CHANGE_BADFLAGS):
				return fmt.Errorf("invalid flags passed for resolution change on %s", monitorDesc)
			case uintptr(DISP_CHANGE_BADPARAM):
				return fmt.Errorf("invalid parameter passed for resolution change on %s", monitorDesc)
			case uintptr(DISP_CHANGE_BADDUALVIEW):
				return fmt.Errorf("unable to change resolution on %s (DualView system)", monitorDesc)
			default:
				lastError = fmt.Errorf("unknown error occurred while changing resolution on %s (error code: %d, system error: %v)", monitorDesc, ret, err)
				continue // retry
			}
		}
	}

	// If we get here, all retries failed
	return fmt.Errorf("failed to change resolution after %d attempts. Last error: %v", maxRetries, lastError)
}

// IsResolutionEqual compares two resolutions for equality
func IsResolutionEqual(r1, r2 Resolution) bool {
	return r1.Width == r2.Width && r1.Height == r2.Height && r1.Frequency == r2.Frequency
}
