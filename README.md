# CS Resolution Monitor

[![Build and Release](https://github.com/ibanks42/csres/actions/workflows/release.yml/badge.svg)](https://github.com/ibanks42/csres/actions/workflows/release.yml)

A Go application that automatically changes desktop resolution when specific applications are running on Windows. This was created so that I could run Counter-Strike 2 in fullscreen windowed mode in 1280x960, making it easier to Alt-Tab between CS2 and other applications.

## Features

- **Automatic Resolution Switching**: Changes screen resolution when monitored applications start
- **Multi-Monitor Support**: Can target specific monitors or use primary monitor
- **Configurable**: Uses JSON configuration file for easy customization
- **Live Configuration Reload**: Automatically reloads configuration when the JSON file is modified
- **Process Monitoring**: Continuously monitors for target applications
- **Graceful Restoration**: Restores default resolution when applications close or on exit
- **Multiple Application Support**: Can monitor multiple applications simultaneously
- **Monitor Detection**: Automatically detects and lists available monitors

## Installation

### Option 1: Download Release (Recommended)
1. Go to the [Releases page](https://github.com/username/csres/releases)
2. Download the latest `csres.exe` from the assets
3. Optionally download `config.example.json` for reference

### Option 2: Build from Source
1. Clone or download this repository
2. Build the application:
   ```bash
   go build -o csres.exe
   ```

## Usage

### First Run

On first run, the application will create a default `config.json` file:

```bash
./csres.exe
```

This creates a sample configuration. Edit `config.json` to match your needs, then run again.

### Running with Custom Config

```bash
./csres.exe [config-file]
```

If no config file is specified, it defaults to `config.json`.

### Configuration

The configuration file has the following structure:

```json
{
  "default_resolution": {
    "width": 1920,
    "height": 1080,
    "frequency": 144
  },
  "default_monitor": "\\\\.\\DISPLAY1",
  "applications": [
    {
      "process_name": "cs2.exe",
      "resolution": {
        "width": 1280,
        "height": 960,
        "frequency": 144
      },
      "monitor_name": "\\\\.\\DISPLAY1"
    }
  ],
  "poll_interval": 2
}
```

#### Configuration Options

- **default_resolution**: The resolution to restore when no monitored applications are running
  - `width`: Screen width in pixels
  - `height`: Screen height in pixels
  - `frequency`: Refresh rate in Hz (optional)

- **default_monitor**: Default monitor for resolution changes (empty = primary monitor)

- **applications**: Array of applications to monitor
  - `process_name`: Exact name of the executable (e.g., "cs2.exe")
  - `resolution`: Target resolution for this application
  - `monitor_name`: Specific monitor to target (empty = primary monitor)

- **poll_interval**: How often to check for running processes (in seconds)

### Monitor Names

Monitor names follow Windows display device naming:
- `""` (empty string): Primary monitor
- `"\\\\.\\DISPLAY1"`: First display device
- `"\\\\.\\DISPLAY2"`: Second display device
- etc.

The application will list all available monitors with their names when it starts. You can see a detailed list by running the application briefly and checking the startup output.

### Example Applications to Monitor

- `cs2.exe` - Counter-Strike 2
- `csgo.exe` - Counter-Strike: Global Offensive

### Live Configuration Changes

You can modify the `config.json` file while the application is running. Changes are automatically detected and applied without restarting the application.

## How It Works

1. **Monitor Detection**: Enumerates available monitors and their current resolutions
2. **Process Monitoring**: Continuously scans running processes every `poll_interval` seconds
3. **Resolution Changes**: When a monitored application starts, changes the specified monitor to its configured resolution
4. **Per-Monitor Tracking**: Tracks resolution changes per monitor, allowing different apps on different monitors
5. **Restoration**: When applications close, restores the default resolution only on monitors that were changed
6. **File Watching**: Monitors the configuration file for changes and reloads automatically
7. **Graceful Shutdown**: Restores default resolution on all changed monitors during Ctrl+C or program termination

## System Requirements

- Windows 10/11
- Go 1.24+ (for building from source)
- Administrator privileges may be required for resolution changes

## Troubleshooting

### Resolution Not Supported Error
If you get a "resolution not supported" error, check that your monitor supports the specified resolution and refresh rate.

### Process Not Detected
- Ensure the process name exactly matches the executable name (case-insensitive)
- Check that the application is actually running in Task Manager
- Some applications may have different executable names than expected

### Permission Errors
- Try running as Administrator if resolution changes fail
- Some display drivers may require elevated privileges

## Building from Source

```bash
go mod tidy
go build -o csres.exe
```
