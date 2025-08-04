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
	ProcessName string     `json:"process_name"` // e.g., "notepad.exe"
	Resolution  Resolution `json:"resolution"`
	MonitorName string     `json:"monitor_name,omitempty"` // Optional: specific monitor name, empty = primary
}

// Config represents the main configuration structure
type Config struct {
	DefaultResolution Resolution  `json:"default_resolution"`        // Resolution to restore when apps close
	DefaultMonitor    string      `json:"default_monitor,omitempty"` // Default monitor name, empty = primary
	Applications      []AppConfig `json:"applications"`              // List of apps and their target resolutions
	PollInterval      int         `json:"poll_interval"`             // Polling interval in seconds (default: 2)
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

	// Set default poll interval if not specified
	if config.PollInterval <= 0 {
		config.PollInterval = 2
	}

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
