package main

import (
	"fmt"
	"log"
)

// getMonitorOptions returns a list of monitor options for the dropdown and a map to convert display names to device names
func (g *GUIApp) getMonitorOptions() ([]string, map[string]string) {
	options := []string{"Primary Monitor"}
	monitorMap := map[string]string{
		"Primary Monitor": "", // Empty string represents primary monitor
	}

	// Create a temporary display manager to get monitor information
	displayManager := NewDisplayManager()

	// Get available monitors
	monitors, err := displayManager.GetAvailableMonitors()
	if err != nil {
		log.Printf("Warning: failed to get monitor list for dropdown: %v", err)
		return options, monitorMap
	}

	// Debug: Print all available monitors
	log.Println("Available monitors:")
	for i, m := range monitors {
		log.Printf("[%d] DeviceName=%s DeviceString=%s IsPrimary=%t", i, m.DeviceName, m.DeviceString, m.IsPrimary)
	}

	// Add each monitor to the options
	for i, monitor := range monitors {
		if monitor.DeviceName != "" {
			// Create a user-friendly display name
			displayName := monitor.DeviceString
			if monitor.IsPrimary {
				displayName += " (Primary)"
			}

			// Add monitor index to make names unique when there are duplicates
			if len(monitors) > 1 {
				displayName += fmt.Sprintf(" [%d]", i+1)
			}

			// Try to get resolution for additional info
			if res, err := displayManager.GetCurrentResolutionForMonitor(monitor.DeviceName); err == nil {
				displayName += fmt.Sprintf(" - %dx%d@%dHz", res.Width, res.Height, res.Frequency)
			}

			options = append(options, displayName)
			monitorMap[displayName] = monitor.DeviceName
		}
	}

	return options, monitorMap
}

// getMonitorDisplayName returns a user-friendly display name for a monitor device name
func (g *GUIApp) getMonitorDisplayName(deviceName string) string {
	if deviceName == "" {
		return "Primary Monitor"
	}

	// Create a temporary display manager to get monitor information
	displayManager := NewDisplayManager()

	// Get available monitors
	monitors, err := displayManager.GetAvailableMonitors()
	if err != nil {
		return deviceName // Fallback to device name
	}

	// Find the monitor with matching device name
	for i, monitor := range monitors {
		if monitor.DeviceName == deviceName {
			displayName := monitor.DeviceString
			if monitor.IsPrimary {
				displayName += " (Primary)"
			}

			// Add monitor index to make names unique when there are duplicates
			if len(monitors) > 1 {
				displayName += fmt.Sprintf(" [%d]", i+1)
			}

			// Try to get resolution for additional info
			if res, err := displayManager.GetCurrentResolutionForMonitor(monitor.DeviceName); err == nil {
				displayName += fmt.Sprintf(" - %dx%d@%dHz", res.Width, res.Height, res.Frequency)
			}

			return displayName
		}
	}

	return deviceName // Fallback if not found
}
