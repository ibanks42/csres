package main

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

// ConfigWatcher handles monitoring of configuration file changes
type ConfigWatcher struct {
	watcher    *fsnotify.Watcher
	configPath string
	configChan chan *Config
	errorChan  chan error
}

// NewConfigWatcher creates a new ConfigWatcher instance
func NewConfigWatcher(configPath string) (*ConfigWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	cw := &ConfigWatcher{
		watcher:    watcher,
		configPath: configPath,
		configChan: make(chan *Config, 1),
		errorChan:  make(chan error, 1),
	}

	// Watch the directory containing the config file
	// This is more reliable than watching the file directly
	configDir := filepath.Dir(configPath)
	if err := watcher.Add(configDir); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to watch config directory: %w", err)
	}

	return cw, nil
}

// Start begins monitoring the configuration file for changes
func (cw *ConfigWatcher) Start() {
	go func() {
		defer cw.watcher.Close()

		for {
			select {
			case event, ok := <-cw.watcher.Events:
				if !ok {
					return
				}

				// Check if the event is for our config file
				if filepath.Clean(event.Name) == filepath.Clean(cw.configPath) {
					// Only respond to write events (file modifications)
					if event.Op&fsnotify.Write == fsnotify.Write {
						log.Printf("Config file modified: %s", event.Name)

						// Load the updated configuration
						config, err := LoadConfig(cw.configPath)
						if err != nil {
							cw.errorChan <- fmt.Errorf("failed to reload config: %w", err)
							continue
						}

						// Send the new config to the channel
						select {
						case cw.configChan <- config:
						default:
							// Channel is full, skip this update
							log.Println("Config channel full, skipping update")
						}
					}
				}

			case err, ok := <-cw.watcher.Errors:
				if !ok {
					return
				}
				cw.errorChan <- fmt.Errorf("file watcher error: %w", err)
			}
		}
	}()
}

// ConfigChan returns the channel that receives updated configurations
func (cw *ConfigWatcher) ConfigChan() <-chan *Config {
	return cw.configChan
}

// ErrorChan returns the channel that receives watcher errors
func (cw *ConfigWatcher) ErrorChan() <-chan error {
	return cw.errorChan
}

// Close closes the file watcher
func (cw *ConfigWatcher) Close() error {
	return cw.watcher.Close()
}
