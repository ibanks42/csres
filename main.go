package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	DefaultConfigFile = "config.json"
)

var (
	Version = "dev" // This will be overridden during build
)

// ResolutionMonitor is the main application structure
type ResolutionMonitor struct {
	config         *Config
	displayManager *DisplayManager
	processMonitor *ProcessMonitor
	configWatcher  *ConfigWatcher
	originalRes    map[string]*Resolution // map of monitor name to original resolution
	currentAppRes  map[string]*Resolution // map of monitor name to current app resolution
	activeApps     map[string]AppConfig
}

// NewResolutionMonitor creates a new ResolutionMonitor instance
func NewResolutionMonitor(configPath string) (*ResolutionMonitor, error) {
	// Load initial configuration
	config, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	// Initialize components
	displayManager := NewDisplayManager()
	processMonitor := NewProcessMonitor()

	// Get available monitors and store original resolutions
	monitors, err := displayManager.GetAvailableMonitors()
	if err != nil {
		return nil, fmt.Errorf("failed to get available monitors: %w", err)
	}

	originalRes := make(map[string]*Resolution)

	// Get original resolution for primary monitor
	primaryRes, err := displayManager.GetCurrentResolution()
	if err != nil {
		return nil, fmt.Errorf("failed to get primary monitor resolution: %w", err)
	}
	originalRes[""] = primaryRes // empty string represents primary monitor

	// Get original resolutions for all monitors
	for _, monitor := range monitors {
		if monitor.DeviceName != "" {
			res, err := displayManager.GetCurrentResolutionForMonitor(monitor.DeviceName)
			if err != nil {
				log.Printf("Warning: failed to get resolution for monitor %s: %v", monitor.DeviceName, err)
				continue
			}
			originalRes[monitor.DeviceName] = res
		}
	}

	// Initialize config watcher
	configWatcher, err := NewConfigWatcher(configPath)
	if err != nil {
		return nil, err
	}

	rm := &ResolutionMonitor{
		config:         config,
		displayManager: displayManager,
		processMonitor: processMonitor,
		configWatcher:  configWatcher,
		originalRes:    originalRes,
		currentAppRes:  make(map[string]*Resolution),
		activeApps:     make(map[string]AppConfig),
	}

	return rm, nil
}

// Start begins the monitoring process
func (rm *ResolutionMonitor) Start() error {
	log.Printf("Starting CS Resolution Monitor v%s...", Version)

	// List available monitors
	monitors, err := rm.displayManager.GetAvailableMonitors()
	if err != nil {
		log.Printf("Warning: failed to get monitor list: %v", err)
	} else {
		log.Printf("Available monitors:")
		for _, monitor := range monitors {
			primaryMarker := ""
			if monitor.IsPrimary {
				primaryMarker = " (Primary)"
			}
			if res, exists := rm.originalRes[monitor.DeviceName]; exists {
				log.Printf("  %s: %s - %dx%d@%dHz%s", monitor.DeviceName, monitor.DeviceString, res.Width, res.Height, res.Frequency, primaryMarker)
			} else {
				log.Printf("  %s: %s%s", monitor.DeviceName, monitor.DeviceString, primaryMarker)
			}
		}
	}

	if primaryRes, exists := rm.originalRes[""]; exists {
		log.Printf("Primary monitor resolution: %dx%d@%dHz", primaryRes.Width, primaryRes.Height, primaryRes.Frequency)
	}
	log.Printf("Default resolution: %dx%d@%dHz", rm.config.DefaultResolution.Width, rm.config.DefaultResolution.Height, rm.config.DefaultResolution.Frequency)

	// Start config file watcher
	rm.configWatcher.Start()

	// Create ticker for process monitoring
	ticker := time.NewTicker(time.Duration(rm.config.PollInterval) * time.Second)
	defer ticker.Stop()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			log.Println("Checking running apps...")
			// Check for running applications
			if err := rm.checkRunningApps(); err != nil {
				log.Printf("Error checking running apps: %v", err)
			}

		case newConfig := <-rm.configWatcher.ConfigChan():
			log.Println("Configuration file updated, reloading...")
			rm.config = newConfig
			// Update ticker interval if changed
			ticker.Stop()
			ticker = time.NewTicker(time.Duration(rm.config.PollInterval) * time.Second)

		case err := <-rm.configWatcher.ErrorChan():
			log.Printf("Config watcher error: %v", err)

		case <-sigChan:
			log.Println("Received shutdown signal...")
			return rm.shutdown()
		}
	}
}

// checkRunningApps monitors for application state changes
func (rm *ResolutionMonitor) checkRunningApps() error {
	runningApps, err := rm.processMonitor.MonitorProcesses(rm.config)
	if err != nil {
		return err
	}

	// Check for newly started applications
	for processName, appConfig := range runningApps {
		if _, exists := rm.activeApps[processName]; !exists {
			log.Printf("Application started: %s", processName)
			if err := rm.handleAppStart(processName, appConfig); err != nil {
				log.Printf("Error handling app start for %s: %v", processName, err)
			}
		}
	}

	// Check for stopped applications
	for processName := range rm.activeApps {
		if _, exists := runningApps[processName]; !exists {
			log.Printf("Application stopped: %s", processName)
			if err := rm.handleAppStop(processName); err != nil {
				log.Printf("Error handling app stop for %s: %v", processName, err)
			}
		}
	}

	rm.activeApps = runningApps
	return nil
}

// handleAppStart changes resolution when a monitored application starts
func (rm *ResolutionMonitor) handleAppStart(processName string, appConfig AppConfig) error {
	monitorName := appConfig.MonitorName
	currentRes, err := rm.displayManager.GetCurrentResolutionForMonitor(monitorName)
	if err != nil {
		return err
	}

	// Only change if the target resolution is different from current
	if !IsResolutionEqual(*currentRes, appConfig.Resolution) {
		monitorDesc := "primary monitor"
		if monitorName != "" {
			monitorDesc = fmt.Sprintf("monitor %s", monitorName)
		}

		log.Printf("Changing resolution to %dx%d@%dHz on %s for %s",
			appConfig.Resolution.Width, appConfig.Resolution.Height, appConfig.Resolution.Frequency, monitorDesc, processName)

		if err := rm.displayManager.ChangeResolutionForMonitor(appConfig.Resolution, monitorName); err != nil {
			return err
		}

		rm.currentAppRes[monitorName] = &appConfig.Resolution
		log.Printf("Resolution changed successfully on %s", monitorDesc)
	}

	return nil
}

// handleAppStop restores default resolution when monitored applications stop
func (rm *ResolutionMonitor) handleAppStop(processName string) error {
	// Find which monitor this app was using
	var appMonitorName string
	for _, app := range rm.config.Applications {
		if app.ProcessName == processName {
			appMonitorName = app.MonitorName
			break
		}
	}

	// Check if any other apps are still using the same monitor
	monitorStillInUse := false
	for _, activeApp := range rm.activeApps {
		if activeApp.MonitorName == appMonitorName && activeApp.ProcessName != processName {
			monitorStillInUse = true
			break
		}
	}

	// If no more apps are using this monitor, restore its default resolution
	if !monitorStillInUse {
		defaultMonitor := rm.config.DefaultMonitor
		if appMonitorName != "" {
			defaultMonitor = appMonitorName
		}

		currentRes, err := rm.displayManager.GetCurrentResolutionForMonitor(defaultMonitor)
		if err != nil {
			return err
		}

		// Only change if current resolution is different from default
		if !IsResolutionEqual(*currentRes, rm.config.DefaultResolution) {
			monitorDesc := "primary monitor"
			if defaultMonitor != "" {
				monitorDesc = fmt.Sprintf("monitor %s", defaultMonitor)
			}

			log.Printf("Restoring default resolution: %dx%d@%dHz on %s",
				rm.config.DefaultResolution.Width, rm.config.DefaultResolution.Height, rm.config.DefaultResolution.Frequency, monitorDesc)

			if err := rm.displayManager.ChangeResolutionForMonitor(rm.config.DefaultResolution, defaultMonitor); err != nil {
				return err
			}

			delete(rm.currentAppRes, defaultMonitor)
			log.Printf("Default resolution restored on %s", monitorDesc)
		}
	}

	return nil
}

// shutdown performs cleanup before exiting
func (rm *ResolutionMonitor) shutdown() error {
	log.Println("Shutting down...")

	// Restore default resolution on all monitors that were changed
	for monitorName := range rm.currentAppRes {
		monitorDesc := "primary monitor"
		if monitorName != "" {
			monitorDesc = fmt.Sprintf("monitor %s", monitorName)
		}

		log.Printf("Restoring default resolution on %s before exit...", monitorDesc)
		if err := rm.displayManager.ChangeResolutionForMonitor(rm.config.DefaultResolution, monitorName); err != nil {
			log.Printf("Error restoring resolution on %s: %v", monitorDesc, err)
		}
	}

	// Close config watcher
	if err := rm.configWatcher.Close(); err != nil {
		log.Printf("Error closing config watcher: %v", err)
	}

	log.Println("Shutdown complete")
	return nil
}

func main() {
	// Handle version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("CS Resolution Monitor v%s\n", Version)
		return
	}

	configFile := DefaultConfigFile
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}

	// Check if config file exists, create default if not
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		log.Printf("Config file %s not found, creating default...", configFile)
		if err := createDefaultConfig(configFile); err != nil {
			log.Fatalf("Failed to create default config: %v", err)
		}
		log.Printf("Default config created at %s. Please edit it and restart the application.", configFile)
		return
	}

	// Create and start monitor
	monitor, err := NewResolutionMonitor(configFile)
	if err != nil {
		log.Fatalf("Failed to create resolution monitor: %v", err)
	}

	if err := monitor.Start(); err != nil {
		log.Fatalf("Monitor error: %v", err)
	}
}

// createDefaultConfig creates a default configuration file
func createDefaultConfig(filename string) error {
	defaultConfig := &Config{
		DefaultResolution: Resolution{
			Width:     1920,
			Height:    1080,
			Frequency: 144,
		},
		DefaultMonitor: "\\\\.\\DISPLAY1",
		Applications: []AppConfig{
			{
				ProcessName: "cs2.exe",
				Resolution: Resolution{
					Width:     1280,
					Height:    960,
					Frequency: 144,
				},
			},
		},
		PollInterval: 2,
	}

	return SaveConfig(defaultConfig, filename)
}
