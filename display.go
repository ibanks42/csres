package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	CCHDEVICENAME                 = 32
	CCHFORMNAME                   = 32
	ENUM_CURRENT_SETTINGS  uint32 = 0xFFFFFFFF
	ENUM_REGISTRY_SETTINGS uint32 = 0xFFFFFFFE
	DISP_CHANGE_SUCCESSFUL uint32 = 0
	DISP_CHANGE_RESTART    uint32 = 1
	DISP_CHANGE_FAILED     uint32 = 0xFFFFFFFF
	DISP_CHANGE_BADMODE    uint32 = 0xFFFFFFFE
)

// DEVMODE is a structure used to specify characteristics of display and print devices
type DEVMODE struct {
	DmDeviceName       [CCHDEVICENAME]uint16
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
	DmFormName         [CCHFORMNAME]uint16
	DmLogPixels        uint16
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
	user32dll                  *syscall.LazyDLL
	procEnumDisplaySettingsW   *syscall.LazyProc
	procChangeDisplaySettingsW *syscall.LazyProc
	procEnumDisplayDevicesW    *syscall.LazyProc
}

// NewDisplayManager creates a new DisplayManager instance
func NewDisplayManager() *DisplayManager {
	user32dll := syscall.NewLazyDLL("user32.dll")
	return &DisplayManager{
		user32dll:                  user32dll,
		procEnumDisplaySettingsW:   user32dll.NewProc("EnumDisplaySettingsW"),
		procChangeDisplaySettingsW: user32dll.NewProc("ChangeDisplaySettingsW"),
		procEnumDisplayDevicesW:    user32dll.NewProc("EnumDisplayDevicesW"),
	}
}

// GetAvailableMonitors returns a list of available monitors
func (dm *DisplayManager) GetAvailableMonitors() ([]MonitorInfo, error) {
	var monitors []MonitorInfo
	var displayDevice DISPLAY_DEVICE
	displayDevice.Cb = uint32(unsafe.Sizeof(displayDevice))

	for i := uint32(0); ; i++ {
		ret, _, _ := dm.procEnumDisplayDevicesW.Call(
			uintptr(unsafe.Pointer(nil)),
			uintptr(i),
			uintptr(unsafe.Pointer(&displayDevice)),
			uintptr(0),
		)

		if ret == 0 {
			break // No more devices
		}

		monitor := MonitorInfo{
			DeviceName:   syscall.UTF16ToString(displayDevice.DeviceName[:]),
			DeviceString: syscall.UTF16ToString(displayDevice.DeviceString[:]),
			IsPrimary:    displayDevice.StateFlags&1 != 0, // DISPLAY_DEVICE_PRIMARY_DEVICE = 1
		}

		monitors = append(monitors, monitor)
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

// ChangeResolution changes the display resolution for primary monitor
func (dm *DisplayManager) ChangeResolution(resolution Resolution) error {
	return dm.ChangeResolutionForMonitor(resolution, "")
}

// ChangeResolutionForMonitor changes the display resolution for a specific monitor
func (dm *DisplayManager) ChangeResolutionForMonitor(resolution Resolution, monitorName string) error {
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

	// Set frequency if specified
	if resolution.Frequency > 0 {
		newMode.DmDisplayFrequency = resolution.Frequency
	}

	// Apply the changes
	ret, _, _ = dm.procChangeDisplaySettingsW.Call(
		deviceNamePtr,
		uintptr(unsafe.Pointer(&newMode)),
		uintptr(0),
	)

	monitorDesc := "primary monitor"
	if monitorName != "" {
		monitorDesc = fmt.Sprintf("monitor %s", monitorName)
	}

	switch ret {
	case uintptr(DISP_CHANGE_SUCCESSFUL):
		return nil
	case uintptr(DISP_CHANGE_RESTART):
		return fmt.Errorf("restart required to apply resolution changes on %s", monitorDesc)
	case uintptr(DISP_CHANGE_BADMODE):
		return fmt.Errorf("resolution %dx%d is not supported by %s", resolution.Width, resolution.Height, monitorDesc)
	case uintptr(DISP_CHANGE_FAILED):
		return fmt.Errorf("failed to change display resolution on %s", monitorDesc)
	default:
		return fmt.Errorf("unknown error occurred while changing resolution on %s", monitorDesc)
	}
}

// IsResolutionEqual compares two resolutions for equality
func IsResolutionEqual(r1, r2 Resolution) bool {
	return r1.Width == r2.Width && r1.Height == r2.Height && r1.Frequency == r2.Frequency
}
