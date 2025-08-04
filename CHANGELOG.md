# Changelog

All notable changes to CS Resolution Monitor will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Multi-monitor support with per-monitor resolution control
- Automatic monitor detection and enumeration
- GitHub Actions for automated building and releases
- Version information in application (`--version` flag)
- Comprehensive error handling with monitor-specific messages
- Support for targeting specific monitors in configuration

### Changed
- Configuration format now includes `monitor_name` and `default_monitor` fields
- Application now tracks resolution changes per monitor
- Enhanced logging with monitor-specific information
- Updated default configuration for Counter-Strike 2

### Fixed
- Improved handling of invalid monitor names
- Better error messages for unsupported resolutions
- Graceful handling of inaccessible monitors

## [1.0.0] - Initial Release

### Added
- Basic resolution monitoring for Windows applications
- JSON configuration file support
- Live configuration reloading
- Process monitoring with configurable poll interval
- Graceful resolution restoration on application exit
- Support for custom refresh rates
- File watching for configuration changes

### Features
- Automatic resolution switching when monitored applications start
- Configurable polling interval for process detection
- Restoration of original resolution when applications close
- Live configuration updates without restart
- Support for multiple applications with different resolutions