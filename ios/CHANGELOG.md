# Changelog

All notable changes to the LCC.live iOS app will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Production-ready infrastructure
- Environment-based configuration system
- Structured logging with OSLog
- Metrics collection service (ready for Grafana integration)
- Network connection monitoring
- Connection status indicator UI
- Memory management with automatic cache pruning
- Privacy manifest (PrivacyInfo.xcprivacy)
- Comprehensive error handling
- CI/CD pipeline with GitHub Actions
- Fastlane automation for TestFlight deployment
- Deployment documentation
- Development setup scripts

### Changed
- Upgraded APIService to use environment configuration
- Improved ImagePreloader with better caching and metrics
- Enhanced error messages throughout the app
- Updated to support iOS 18.0+
- Improved logging from DEBUG prints to structured OSLog

### Fixed
- Network timeout handling
- Memory warnings now properly handled
- Image cache now has size limits

## [1.0.0] - TBD

### Added
- Initial release
- Two-tab interface (LCC and BCC)
- Live camera feed images
- YouTube video embed support
- Grid layout with compact/single modes
- Fullscreen image gallery with swipe navigation
- Pull-to-refresh
- Auto-refresh every 5 seconds
- Image preloading and caching
- Modern tab bar
- Smooth animations and transitions
- Support for iPhone and iPad

### Technical
- SwiftUI-based interface
- MVVM architecture
- Combine for reactive programming
- URLSession for networking
- WKWebView for YouTube embeds
- Environment-based configuration
- Comprehensive test suite

## Version History Format

### [Version] - YYYY-MM-DD

#### Added
- New features

#### Changed
- Changes to existing functionality

#### Deprecated
- Soon-to-be removed features

#### Removed
- Removed features

#### Fixed
- Bug fixes

#### Security
- Security improvements

---

## Release Notes Template

Use this template for future releases:

```markdown
## [X.Y.Z] - YYYY-MM-DD

### What's New
- Brief description of main features

### Improvements
- List of enhancements

### Bug Fixes
- List of fixes

### Known Issues
- Any known issues

### Upgrade Notes
- Special instructions if needed
```

