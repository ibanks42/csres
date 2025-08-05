package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Resolution represents screen resolution settings
type Resolution struct {
	Width     uint32 `json:"width"`
	Height    uint32 `json:"height"`
	Frequency uint32 `json:"frequency,omitempty"` // Optional refresh rate
}

// AppConfig represents configuration for a specific application
type AppConfig struct {
	ProcessName       string      `json:"process_name"` // e.g., "notepad.exe"
	Resolution        Resolution  `json:"resolution"`
	MonitorName       string      `json:"monitor_name"`                 // Required: specific monitor name, empty = primary
	RestoreResolution *Resolution `json:"restore_resolution,omitempty"` // Optional: resolution to restore to when app closes. If nil, uses original resolution
}

// Config represents the main configuration structure
type Config struct {
	Applications        []AppConfig `json:"applications"`          // List of apps and their target resolutions
	PollInterval        int         `json:"poll_interval"`         // Polling interval in seconds (default: 2)
	ShowGUIOnLaunch     bool        `json:"show_gui_on_launch"`    // Show GUI window on launch (default: true)
	StartWithWindows    bool        `json:"start_with_windows"`    // Start with Windows (default: false)
	AutoStartMonitoring bool        `json:"auto_start_monitoring"` // Auto-start monitoring on launch (default: true)
}

// LoadConfig loads configuration from a JSON file
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Set default values if not specified
	if config.PollInterval <= 0 {
		config.PollInterval = 2
	}

	// Set defaults for new fields if this is an existing config file
	// ShowGUIOnLaunch defaults to true if not set
	// StartWithWindows defaults to false
	// AutoStartMonitoring defaults to true

	return &config, nil
}

// SaveConfig saves configuration to a JSON file (useful for creating default config)
func SaveConfig(config *Config, filename string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
