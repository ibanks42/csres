package main

import (
	"fmt"
	"strconv"
	"strings"
)

// parseAppInfo parses the application info string back to AppConfig
func (g *GUIApp) parseAppInfo(appInfo string) (AppConfig, error) {
	// Format: "cs2.exe - 1280x960@144Hz (Primary Monitor)"
	// or: "cs2.exe - 1280x960@144Hz (Intel(R) Iris(R) Xe Graphics (Primary) - 2880x1800@120Hz)"

	// First extract the device name if it exists (after null byte)
	parts := strings.Split(appInfo, "\x00")
	var deviceName string
	if len(parts) > 1 {
		deviceName = parts[1]
		appInfo = parts[0]
	}

	// Find the process name - it's everything before the first " - "
	firstDashIndex := strings.Index(appInfo, " - ")
	if firstDashIndex == -1 {
		return AppConfig{}, fmt.Errorf("invalid application info format")
	}

	processName := appInfo[:firstDashIndex]
	remainder := appInfo[firstDashIndex+3:] // Skip " - "

	// Find the monitor part - it's enclosed in the FIRST set of parentheses after the resolution
	// We need to be careful because monitor names can contain multiple parentheses
	firstOpenParen := strings.Index(remainder, " (")
	if firstOpenParen == -1 {
		return AppConfig{}, fmt.Errorf("invalid resolution and monitor format")
	}

	resolutionStr := remainder[:firstOpenParen]
	// Find the matching closing parenthesis
	openCount := 1
	closeParenPos := -1
	for i := firstOpenParen + 2; i < len(remainder); i++ {
		if remainder[i] == '(' {
			openCount++
		} else if remainder[i] == ')' {
			openCount--
			if openCount == 0 {
				closeParenPos = i
				break
			}
		}
	}
	if closeParenPos == -1 {
		return AppConfig{}, fmt.Errorf("invalid monitor format - missing closing parenthesis")
	}
	monitorStr := remainder[firstOpenParen+2 : closeParenPos]

	// Find restore resolution if present
	var restoreResolution *Resolution
	if restoreIdx := strings.Index(monitorStr, "[Restore: "); restoreIdx != -1 {
		restoreEnd := strings.Index(monitorStr[restoreIdx:], "]")
		if restoreEnd != -1 {
			restoreStr := monitorStr[restoreIdx+len("[Restore: ") : restoreIdx+restoreEnd]
			if restoreStr != "default" {
				res, err := parseResolutionString(restoreStr)
				if err == nil {
					restoreResolution = &res
				}
			}
			monitorStr = strings.TrimSpace(monitorStr[:restoreIdx])
		}
	}

	// Parse resolution: "1280x960@144Hz"
	resolution, err := parseResolutionString(resolutionStr)
	if err != nil {
		return AppConfig{}, fmt.Errorf("failed to parse resolution: %w", err)
	}

	// Use the stored device name if we have it, otherwise convert from display name
	var monitorName string
	if deviceName != "" {
		monitorName = strings.TrimSpace(deviceName)
	} else {
		// Fallback to converting display name if no stored device name
		monitorName = strings.TrimSpace(g.getDeviceNameFromDisplayName(monitorStr))
	}

	// Empty string and Primary Monitor are equivalent
	if monitorName == "Primary Monitor" {
		monitorName = ""
	}

	return AppConfig{
		ProcessName:       processName,
		Resolution:        resolution,
		MonitorName:       monitorName,
		RestoreResolution: restoreResolution,
	}, nil
}

// parseResolutionString parses a resolution string like "1280x960@144Hz"
func parseResolutionString(resStr string) (Resolution, error) {
	// Split by "@" to separate resolution and frequency
	parts := strings.Split(resStr, "@")
	if len(parts) != 2 {
		return Resolution{}, fmt.Errorf("invalid resolution format (expected WIDTHxHEIGHT@FREQHz)")
	}

	// Parse width and height
	dimParts := strings.Split(parts[0], "x")
	if len(dimParts) != 2 {
		return Resolution{}, fmt.Errorf("invalid dimension format (expected WIDTHxHEIGHT)")
	}

	width, err := strconv.ParseUint(dimParts[0], 10, 32)
	if err != nil {
		return Resolution{}, fmt.Errorf("invalid width: %w", err)
	}

	height, err := strconv.ParseUint(dimParts[1], 10, 32)
	if err != nil {
		return Resolution{}, fmt.Errorf("invalid height: %w", err)
	}

	// Parse frequency (remove "Hz" suffix)
	freqStr := strings.TrimSuffix(parts[1], "Hz")
	frequency, err := strconv.ParseUint(freqStr, 10, 32)
	if err != nil {
		return Resolution{}, fmt.Errorf("invalid frequency: %w", err)
	}

	return Resolution{
		Width:     uint32(width),
		Height:    uint32(height),
		Frequency: uint32(frequency),
	}, nil
}

// getDeviceNameFromDisplayName converts a monitor display name back to its device name
func (g *GUIApp) getDeviceNameFromDisplayName(displayName string) string {
	if displayName == "Primary Monitor" {
		return "" // Empty string represents primary monitor
	}

	// Create a temporary display manager to get monitor information
	displayManager := NewDisplayManager()

	// Get available monitors
	monitors, err := displayManager.GetAvailableMonitors()
	if err != nil {
		return displayName // Fallback to display name if we can't get monitor info
	}

	// Clean up the display name for comparison
	cleanDisplayName := displayName
	// Remove resolution info if present
	if idx := strings.LastIndex(cleanDisplayName, " - "); idx != -1 {
		cleanDisplayName = cleanDisplayName[:idx]
	}
	// Remove index if present
	if idx := strings.LastIndex(cleanDisplayName, " ["); idx != -1 {
		cleanDisplayName = cleanDisplayName[:idx]
	}

	// Find the monitor that matches this display name
	for _, monitor := range monitors {
		if monitor.DeviceName != "" {
			// Recreate the display name for this monitor
			testDisplayName := monitor.DeviceString
			if monitor.IsPrimary {
				testDisplayName += " (Primary)"
			}

			// Check if this matches our target display name
			if testDisplayName == cleanDisplayName {
				return monitor.DeviceName
			}
		}
	}

	// If still no match found, return empty string for "Primary Monitor" or the device name as fallback
	if cleanDisplayName == "Primary Monitor" {
		return ""
	}
	return displayName
}
