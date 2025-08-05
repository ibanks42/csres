package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"

	"fyne.io/fyne/v2/widget"
)

// GUIApp represents the GUI application
type GUIApp struct {
	app                      fyne.App
	mainWindow               fyne.Window
	resMonitor               *ResolutionMonitor
	displayManager           *DisplayManager
	configPath               string
	ctx                      context.Context
	cancel                   context.CancelFunc
	appList                  *widget.List
	appData                  binding.StringList
	pollEntry                *widget.Entry
	statusLabel              *widget.Label
	startStopBtn             *widget.Button
	showGUICheck             *widget.Check
	startWithWindowsCheck    *widget.Check
	autoStartMonitoringCheck *widget.Check
	isRunning                bool
	configWatcher            *ConfigWatcher
}

// NewGUIApp creates a new GUI application
func NewGUIApp(configPath string) *GUIApp {
	fyneApp := app.NewWithID("com.csres.monitor")

	ctx, cancel := context.WithCancel(context.Background())

	gui := &GUIApp{
		app:            fyneApp,
		configPath:     configPath,
		ctx:            ctx,
		cancel:         cancel,
		appData:        binding.NewStringList(),
		isRunning:      false,
		displayManager: NewDisplayManager(),
	}

	return gui
}

// Run starts the GUI application
func (g *GUIApp) Run() error {
	// Set up system tray
	if desk, ok := g.app.(desktop.App); ok {
		g.setupSystemTray(desk)
	}

	// Create main window but don't show it initially
	g.createMainWindow()

	// Load initial configuration
	if err := g.loadConfig(); err != nil {
		dialog.ShowError(err, nil)
		return err
	}

	// Show or hide main window based on configuration
	config, _ := LoadConfig(g.configPath)
	if config != nil && config.ShowGUIOnLaunch {
		g.showMainWindow()
	}

	// Auto-start monitoring if configured
	if config != nil && config.AutoStartMonitoring {
		g.startMonitoring()
	}

	// Start the resolution monitor in a goroutine
	go g.runResolutionMonitor()

	// Set up config file watcher
	watcher, err := NewConfigWatcher(g.configPath)
	if err != nil {
		log.Printf("Warning: Failed to create config watcher: %v", err)
	} else {
		g.configWatcher = watcher
		watcher.Start()

		// Handle config updates
		go func() {
			for {
				select {
				case <-g.ctx.Done():
					return
				case <-watcher.ConfigChan():
					// Run on main thread since we're updating UI
					fyne.Do(func() {
						g.reloadConfig()
					})
				case err := <-watcher.ErrorChan():
					log.Printf("Config watcher error: %v", err)
				}
			}
		}()
	}

	// Run the app (this blocks)
	g.app.Run()

	return nil
}

// setupSystemTray configures the system tray icon and menu
func (g *GUIApp) setupSystemTray(desk desktop.App) {
	// Create tray menu
	menu := fyne.NewMenu("CS Resolution Monitor",
		fyne.NewMenuItem("Show", func() {
			g.showMainWindow()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Toggle Monitoring", func() {
			g.toggleMonitoring()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Quit", func() {
			g.quit()
		}),
	)

	// Set up system tray
	desk.SetSystemTrayMenu(menu)
	desk.SetSystemTrayIcon(resourceIconPng) // We'll need to create this resource
}

// createMainWindow creates the main configuration window
func (g *GUIApp) createMainWindow() {
	window := g.app.NewWindow("CS Resolution Monitor")
	window.Resize(fyne.NewSize(600, 500))
	window.SetCloseIntercept(func() {
		window.Hide() // Hide instead of close
	})

	// Status section
	g.statusLabel = widget.NewLabel("Status: Stopped")
	g.startStopBtn = widget.NewButton("Start Monitoring", func() {
		g.toggleMonitoring()
	})

	statusContainer := container.NewHBox(
		g.statusLabel,
		layout.NewSpacer(),
		g.startStopBtn,
	)

	// Application list section
	appLabel := widget.NewLabel("Monitored Applications:")

	g.appList = widget.NewListWithData(
		g.appData,
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel(""),
				layout.NewSpacer(),
				widget.NewButton("Edit", nil),
				widget.NewButton("Delete", nil),
			)
		},
		func(item binding.DataItem, obj fyne.CanvasObject) {
			cont := obj.(*fyne.Container)
			label := cont.Objects[0].(*widget.Label)
			editBtn := cont.Objects[2].(*widget.Button)
			deleteBtn := cont.Objects[3].(*widget.Button)

			if stringItem, ok := item.(binding.String); ok {
				appInfo, _ := stringItem.Get()
				label.SetText(appInfo)

				// Set button callbacks with the current item
				editBtn.OnTapped = func() {
					g.editApplication(appInfo)
				}
				deleteBtn.OnTapped = func() {
					g.deleteApplication(appInfo)
				}
			}
		},
	)

	addAppBtn := widget.NewButton("Add Application", func() {
		g.addApplication()
	})

	// Application container with header and list that expands
	appContainer := container.NewBorder(
		// Top: Label and Add button
		container.NewVBox(appLabel, addAppBtn),
		// Bottom: nil
		nil,
		// Left: nil
		nil,
		// Right: nil
		nil,
		// Center: List (expands to fill space)
		g.appList,
	)

	// Settings section
	settingsLabel := widget.NewLabel("Settings:")
	pollLabel := widget.NewLabel("Poll Interval (seconds):")
	g.pollEntry = widget.NewEntry()
	g.pollEntry.SetText("2")

	// New settings checkboxes
	g.showGUICheck = widget.NewCheck("Show GUI on launch", nil)
	g.startWithWindowsCheck = widget.NewCheck("Start with Windows", nil)
	g.autoStartMonitoringCheck = widget.NewCheck("Auto-start monitoring", nil)

	saveSettingsBtn := widget.NewButton("Save Settings", func() {
		g.saveSettings()
	})

	settingsContainer := container.NewVBox(
		settingsLabel,
		container.NewHBox(pollLabel, g.pollEntry),
		g.showGUICheck,
		g.startWithWindowsCheck,
		g.autoStartMonitoringCheck,
		saveSettingsBtn,
	)

	// Main layout - use border container to make app list fill space and push settings to bottom
	content := container.NewBorder(
		// Top: Status section with separator
		container.NewVBox(statusContainer, widget.NewSeparator()),
		// Bottom: Settings section with separator
		container.NewVBox(widget.NewSeparator(), settingsContainer),
		// Left: nil
		nil,
		// Right: nil
		nil,
		// Center: Application list (will expand to fill available space)
		appContainer,
	)

	window.SetContent(content)
	g.mainWindow = window
}

// showMainWindow shows the main configuration window
func (g *GUIApp) showMainWindow() {
	if g.mainWindow != nil {
		g.mainWindow.Show()
		g.mainWindow.RequestFocus()
	}
}

// loadConfig loads the configuration and updates the GUI
func (g *GUIApp) loadConfig() error {
	// Check if config file exists, create default if not
	if _, err := os.Stat(g.configPath); os.IsNotExist(err) {
		log.Printf("Config file %s not found, creating default...", g.configPath)
		if err := createDefaultConfig(g.configPath); err != nil {
			return fmt.Errorf("failed to create default config: %w", err)
		}
	}

	// Load configuration
	config, err := LoadConfig(g.configPath)
	if err != nil {
		return err
	}

	// Update GUI with loaded config
	fyne.Do(func() {
		g.updateAppList(config)
		g.pollEntry.SetText(fmt.Sprintf("%d", config.PollInterval))

		// Update checkbox states if they exist
		if g.showGUICheck != nil {
			g.showGUICheck.SetChecked(config.ShowGUIOnLaunch)
		}
		if g.startWithWindowsCheck != nil {
			g.startWithWindowsCheck.SetChecked(config.StartWithWindows)
		}
		if g.autoStartMonitoringCheck != nil {
			g.autoStartMonitoringCheck.SetChecked(config.AutoStartMonitoring)
		}
		g.mainWindow.Content().Refresh()
	})

	return nil
}

// updateAppList updates the application list in the GUI
func (g *GUIApp) updateAppList(config *Config) {
	// Clear existing items
	g.appData.Set([]string{})

	// Add applications from config
	for _, app := range config.Applications {
		monitor := g.getMonitorDisplayName(app.MonitorName)

		// Store the device name in the UI string (hidden) after a null byte so it won't be visible
		restoreInfo := "default"
		if app.RestoreResolution != nil {
			restoreInfo = fmt.Sprintf("%dx%d@%dHz",
				app.RestoreResolution.Width,
				app.RestoreResolution.Height,
				app.RestoreResolution.Frequency)
		}
		appInfo := fmt.Sprintf("%s - %dx%d@%dHz (%s) [Restore: %s]\x00%s",
			app.ProcessName,
			app.Resolution.Width,
			app.Resolution.Height,
			app.Resolution.Frequency,
			monitor,
			restoreInfo,
			app.MonitorName)

		g.appData.Append(appInfo)
	}
}

// addApplication shows dialog to add a new application
func (g *GUIApp) addApplication() {
	g.showAppDialog(AppConfig{}, false)
}

// editApplication shows dialog to edit an existing application
func (g *GUIApp) editApplication(appInfo string) {
	// Parse appInfo to get the original AppConfig
	app, err := g.parseAppInfo(appInfo)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to parse application info: %w", err), g.mainWindow)
		return
	}

	g.showAppDialog(app, true)
}

// deleteApplication removes an application from monitoring
func (g *GUIApp) deleteApplication(appInfo string) {
	dialog.ShowConfirm("Delete Application",
		fmt.Sprintf("Are you sure you want to remove this application from monitoring?\n\n%s", appInfo),
		func(confirmed bool) {
			if confirmed {
				// Parse appInfo to get the process name
				app, err := g.parseAppInfo(appInfo)
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to parse application info: %w", err), g.mainWindow)
					return
				}

				// Load current config
				config, err := LoadConfig(g.configPath)
				if err != nil {
					dialog.ShowError(err, g.mainWindow)
					return
				}

				// Remove the application from config
				newApps := []AppConfig{}
				for _, configApp := range config.Applications {
					// Check if this is the app to delete - use a more robust matching
					// Normalize monitor names for comparison
					configMonitor := strings.TrimSpace(configApp.MonitorName)
					compareMonitor := strings.TrimSpace(app.MonitorName)

					// Empty string and Primary Monitor are equivalent
					if configMonitor == "Primary Monitor" {
						configMonitor = ""
					}
					if compareMonitor == "Primary Monitor" {
						compareMonitor = ""
					}

					// Match by process name and resolution (most reliable identifiers)
					isMatch := configApp.ProcessName == app.ProcessName &&
						configApp.Resolution.Width == app.Resolution.Width &&
						configApp.Resolution.Height == app.Resolution.Height &&
						configApp.Resolution.Frequency == app.Resolution.Frequency

					// If monitor names are specified, also match by monitor
					if configMonitor != "" || compareMonitor != "" {
						isMatch = isMatch && configMonitor == compareMonitor
					}

					// Keep all apps that DON'T match (i.e., delete the one that matches)
					if !isMatch {
						newApps = append(newApps, configApp)
					}
				}
				config.Applications = newApps

				// Save config
				if err := SaveConfig(config, g.configPath); err != nil {
					dialog.ShowError(err, g.mainWindow)
					return
				}

				// Update resolution monitor config if it exists
				if g.resMonitor != nil {
					g.resMonitor.config = config
				}

				// Reload GUI
				g.reloadConfig()
			}
		}, g.mainWindow)
}

// showAppDialog shows the add/edit application dialog
func (g *GUIApp) showAppDialog(app AppConfig, isEdit bool) {
	// Store the original app config for edit comparisons - make a copy to avoid modification
	var originalApp *AppConfig
	if isEdit {
		monitorName := strings.TrimSpace(app.MonitorName)
		if monitorName == "Primary Monitor" {
			monitorName = ""
		}
		originalApp = &AppConfig{
			ProcessName: app.ProcessName,
			MonitorName: monitorName,
			Resolution: Resolution{
				Width:     app.Resolution.Width,
				Height:    app.Resolution.Height,
				Frequency: app.Resolution.Frequency,
			},
			RestoreResolution: app.RestoreResolution,
		}
	}
	title := "Add Application"
	if isEdit {
		title = "Edit Application"
	}

	// Create form fields
	processEntry := widget.NewEntry()
	processEntry.SetPlaceHolder("e.g., cs2.exe")
	if app.ProcessName != "" {
		processEntry.SetText(app.ProcessName)
	}

	// Create monitor dropdown
	monitorOptions, monitorMap := g.getMonitorOptions()
	monitorSelect := widget.NewSelect(monitorOptions, nil)

	// Create resolution dropdown
	var resolutionOptions []string
	resolutionMap := make(map[string]Resolution)
	resolutionSelect := widget.NewSelect(resolutionOptions, nil)

	// Create restore resolution dropdown
	var restoreResolutionOptions []string
	restoreResolutionMap := make(map[string]Resolution)
	restoreResolutionSelect := widget.NewSelect(restoreResolutionOptions, nil)

	// Function to update resolution options based on selected monitor
	updateResolutionOptions := func(monitorName string) {
		resolutions, err := g.displayManager.GetAvailableResolutions(monitorName)
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to get available resolutions: %v", err), g.mainWindow)
			return
		}

		// Clear existing options
		resolutionOptions = nil
		resolutionMap = make(map[string]Resolution)
		restoreResolutionOptions = nil
		restoreResolutionMap = make(map[string]Resolution)

		// Add "Current Resolution" option for restore
		if currentRes, err := g.displayManager.GetCurrentResolutionForMonitor(monitorName); err == nil {
			restoreResolutionStr := fmt.Sprintf("Current (%dx%d@%dHz)", currentRes.Width, currentRes.Height, currentRes.Frequency)
			restoreResolutionOptions = append(restoreResolutionOptions, restoreResolutionStr)
			restoreResolutionMap[restoreResolutionStr] = *currentRes
		}

		// Add available resolutions in reverse order (typically higher resolutions first)
		for i := len(resolutions) - 1; i >= 0; i-- {
			res := resolutions[i]
			resStr := fmt.Sprintf("%dx%d@%dHz", res.Width, res.Height, res.Frequency)
			resolutionOptions = append(resolutionOptions, resStr)
			resolutionMap[resStr] = res

			// Also add to restore options
			restoreResolutionOptions = append(restoreResolutionOptions, resStr)
			restoreResolutionMap[resStr] = res
		}

		resolutionSelect.Options = resolutionOptions
		restoreResolutionSelect.Options = restoreResolutionOptions

		// Set target resolution selection
		if app.Resolution.Width > 0 {
			// Try to find the same resolution (prioritizing highest frequency)
			targetResStr := fmt.Sprintf("%dx%d@%dHz", app.Resolution.Width, app.Resolution.Height, app.Resolution.Frequency)
			found := false

			// First try exact match
			for _, option := range resolutionOptions {
				if option == targetResStr {
					resolutionSelect.SetSelected(option)
					found = true
					break
				}
			}

			// If no exact match, try to find same resolution with different frequency (prioritize highest)
			if !found {
				highestFreq := uint32(0)
				var bestMatch string
				for _, option := range resolutionOptions {
					if res, exists := resolutionMap[option]; exists {
						if res.Width == app.Resolution.Width && res.Height == app.Resolution.Height {
							if res.Frequency > highestFreq {
								highestFreq = res.Frequency
								bestMatch = option
							}
						}
					}
				}
				if bestMatch != "" {
					resolutionSelect.SetSelected(bestMatch)
					found = true
				}
			}

			// If still no match, select highest resolution available
			if !found && len(resolutionOptions) > 0 {
				resolutionSelect.SetSelected(resolutionOptions[0])
			}
		} else if len(resolutionOptions) > 0 {
			resolutionSelect.SetSelected(resolutionOptions[0])
		}

		// Set restore resolution selection - always select highest available when monitor changes
		if len(restoreResolutionOptions) > 0 {
			restoreResolutionSelect.SetSelected(restoreResolutionOptions[0])
		}
	}

	// Set up monitor selection callback
	monitorSelect.OnChanged = func(selected string) {
		updateResolutionOptions(monitorMap[selected])
	}

	// Set initial monitor selection
	if app.MonitorName == "" {
		monitorSelect.SetSelected("Primary Monitor")
	} else {
		// Convert device name back to display name for selection
		displayName := g.getMonitorDisplayName(app.MonitorName)

		// Try to find exact match first
		found := false
		for _, option := range monitorOptions {
			if option == displayName {
				monitorSelect.SetSelected(option)
				found = true
				break
			}
		}

		// If no exact match, try to find by device name
		if !found {
			for displayStr, deviceName := range monitorMap {
				if deviceName == app.MonitorName {
					monitorSelect.SetSelected(displayStr)
					break
				}
			}
		}
	}

	// Initial resolution options update
	updateResolutionOptions(monitorMap[monitorSelect.Selected])

	// Create form
	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Process Name:", Widget: processEntry},
			{Text: "Monitor:", Widget: monitorSelect},
			{Text: "Target Resolution:", Widget: resolutionSelect},
			{Text: "Restore Resolution:", Widget: restoreResolutionSelect},
		},
	}

	// Create dialog
	d := dialog.NewCustomConfirm(title, "Save", "Cancel", form, func(confirmed bool) {
		if confirmed {
			selectedMonitor := monitorMap[monitorSelect.Selected]
			selectedResolution := resolutionMap[resolutionSelect.Selected]
			selectedRestoreResolution := restoreResolutionMap[restoreResolutionSelect.Selected]

			if isEdit {
				g.saveApplication(processEntry.Text, selectedResolution, selectedRestoreResolution, selectedMonitor, originalApp)
			} else {
				g.saveApplication(processEntry.Text, selectedResolution, selectedRestoreResolution, selectedMonitor, nil)
			}
		}
	}, g.mainWindow)

	d.Resize(fyne.NewSize(450, 350))
	d.Show()
}

// saveApplication saves a new or edited application configuration
func (g *GUIApp) saveApplication(process string, resolution, restoreResolution Resolution, monitor string, originalApp *AppConfig) {
	// Validate inputs
	if process == "" {
		dialog.ShowError(fmt.Errorf("process name is required"), g.mainWindow)
		return
	}

	// Load current config
	config, err := LoadConfig(g.configPath)
	if err != nil {
		dialog.ShowError(err, g.mainWindow)
		return
	}

	// Create new app config
	newApp := AppConfig{
		ProcessName:       process,
		Resolution:        resolution,
		MonitorName:       monitor, // This should be the device name from monitorMap
		RestoreResolution: &restoreResolution,
	}

	// If editing, remove the original entry first
	if originalApp != nil {
		// Convert display name back to device name for comparison
		deviceName := g.getDeviceNameFromDisplayName(originalApp.MonitorName)

		// Remove the original entry completely
		newApps := []AppConfig{}
		for _, configApp := range config.Applications {
			// Check if this is the original app to remove - use a more robust matching
			// Normalize monitor names for comparison
			configMonitor := strings.TrimSpace(configApp.MonitorName)
			compareMonitor := strings.TrimSpace(deviceName)

			// Empty string and Primary Monitor are equivalent
			if configMonitor == "Primary Monitor" {
				configMonitor = ""
			}
			if compareMonitor == "Primary Monitor" {
				compareMonitor = ""
			}

			// Match by process name and resolution (most reliable identifiers)
			isOriginal := configApp.ProcessName == originalApp.ProcessName &&
				configApp.Resolution.Width == originalApp.Resolution.Width &&
				configApp.Resolution.Height == originalApp.Resolution.Height &&
				configApp.Resolution.Frequency == originalApp.Resolution.Frequency

			// If monitor names are specified, also match by monitor
			if configMonitor != "" || compareMonitor != "" {
				isOriginal = isOriginal && configMonitor == compareMonitor
			}

			// Keep all apps that are NOT the original
			if !isOriginal {
				newApps = append(newApps, configApp)
			}
		}
		config.Applications = newApps
	}

	// Always add the new application (whether it's a new entry or an edit)
	config.Applications = append(config.Applications, newApp)

	// Save config
	if err := SaveConfig(config, g.configPath); err != nil {
		dialog.ShowError(err, g.mainWindow)
		return
	}

	// Update resolution monitor config if it exists
	if g.resMonitor != nil {
		g.resMonitor.config = config
	}

	// Reload GUI
	g.reloadConfig()
}

// saveSettings saves the current settings
func (g *GUIApp) saveSettings() {
	pollInterval, err := strconv.Atoi(g.pollEntry.Text)
	if err != nil || pollInterval < 1 {
		dialog.ShowError(fmt.Errorf("poll interval must be a positive number"), g.mainWindow)
		return
	}

	// Load current config
	config, err := LoadConfig(g.configPath)
	if err != nil {
		dialog.ShowError(err, g.mainWindow)
		return
	}

	// Update all settings
	config.PollInterval = pollInterval
	config.ShowGUIOnLaunch = g.showGUICheck.Checked
	config.StartWithWindows = g.startWithWindowsCheck.Checked
	config.AutoStartMonitoring = g.autoStartMonitoringCheck.Checked

	// Handle Windows startup setting
	if err := g.handleWindowsStartup(config.StartWithWindows); err != nil {
		dialog.ShowError(fmt.Errorf("failed to update Windows startup setting: %w", err), g.mainWindow)
		return
	}

	// Save config
	if err := SaveConfig(config, g.configPath); err != nil {
		dialog.ShowError(err, g.mainWindow)
		return
	}

	// Update resolution monitor config if it exists
	if g.resMonitor != nil {
		g.resMonitor.config = config
	}

	dialog.ShowInformation("Settings Saved", "Settings have been saved successfully.", g.mainWindow)
}

// reloadConfig reloads the configuration and updates the GUI
func (g *GUIApp) reloadConfig() {
	if err := g.loadConfig(); err != nil {
		dialog.ShowError(err, g.mainWindow)
	}
}

// toggleMonitoring toggles between start and stop monitoring
func (g *GUIApp) toggleMonitoring() {
	if g.isRunning {
		g.stopMonitoring()
	} else {
		g.startMonitoring()
	}
}

// startMonitoring starts the resolution monitoring
func (g *GUIApp) startMonitoring() {
	if g.isRunning {
		return
	}

	// Create resolution monitor if not exists
	if g.resMonitor == nil {
		monitor, err := NewResolutionMonitor(g.configPath)
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to create resolution monitor: %w", err), g.mainWindow)
			return
		}
		g.resMonitor = monitor
	}

	g.isRunning = true
	g.statusLabel.SetText("Status: Running")
	if g.startStopBtn != nil {
		g.startStopBtn.SetText("Stop Monitoring")
	}
	log.Println("GUI: Starting monitoring...")
}

// stopMonitoring stops the resolution monitoring
func (g *GUIApp) stopMonitoring() {
	if !g.isRunning {
		return
	}

	g.isRunning = false
	g.statusLabel.SetText("Status: Stopped")
	if g.startStopBtn != nil {
		g.startStopBtn.SetText("Start Monitoring")
	}

	// Restore original resolutions
	if g.resMonitor != nil {
		for monitorName := range g.resMonitor.currentAppRes {
			monitorDesc := "primary monitor"
			if monitorName != "" {
				monitorDesc = fmt.Sprintf("monitor %s", monitorName)
			}

			originalRes, exists := g.resMonitor.originalRes[monitorName]
			if !exists {
				log.Printf("Warning: no original resolution stored for %s", monitorDesc)
				continue
			}

			log.Printf("GUI: Restoring original resolution on %s...", monitorDesc)
			if err := g.resMonitor.displayManager.SetResolution(monitorName, *originalRes); err != nil {
				log.Printf("Error restoring resolution on %s: %v", monitorDesc, err)
			}
		}
		// Clear active apps to reset state
		g.resMonitor.activeApps = make(map[string]AppConfig)
		g.resMonitor.currentAppRes = make(map[string]*Resolution)
	}

	log.Println("GUI: Monitoring stopped")
}

// runResolutionMonitor runs the resolution monitor in the background
func (g *GUIApp) runResolutionMonitor() {
	ticker := time.NewTicker(2 * time.Second) // Default polling interval
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			if g.isRunning && g.resMonitor != nil {
				// Check for running applications
				if err := g.resMonitor.checkRunningApps(); err != nil {
					log.Printf("GUI: Error checking running apps: %v", err)
				}

				// Update ticker interval if config changed
				if g.resMonitor.config.PollInterval > 0 {
					newInterval := time.Duration(g.resMonitor.config.PollInterval) * time.Second
					if ticker.C != nil { // Recreate ticker if interval changed
						ticker.Stop()
						ticker = time.NewTicker(newInterval)
					}
				}
			}
		}
	}
}

// quit gracefully shuts down the application
func (g *GUIApp) quit() {
	log.Println("GUI: Shutting down...")
	g.cancel()
	if g.resMonitor != nil {
		g.resMonitor.shutdown()
	}
	if g.configWatcher != nil {
		if err := g.configWatcher.Close(); err != nil {
			log.Printf("Error closing config watcher: %v", err)
		}
	}
	g.app.Quit()
}

//go:embed icon.png
var resourceIconPngBytes []byte

var resourceIconPng = &fyne.StaticResource{
	StaticName:    "icon.png",
	StaticContent: resourceIconPngBytes,
}
