# LCC.live iOS App

> Live camera feeds for Utah's Little & Big Cottonwood Canyons

![Version](https://img.shields.io/badge/version-1.0.0-blue)
![iOS](https://img.shields.io/badge/iOS-26.0+-blue)
![Swift](https://img.shields.io/badge/Swift-5.9-orange)

## Features

- 📷 Real-time camera feeds (auto-refresh every 5s)
- 🎥 YouTube video embeds
- 📱 Dual canyon tabs (LCC / BCC)
- 🔍 Fullscreen gallery with swipe navigation
- 📡 Connection status indicator
- 🌙 Dark mode support
- ⚡ Smart image caching & memory management

## Quick Start

```bash
# 1. Setup (installs dependencies)
./scripts/setup_env.sh

# 2. Open in Xcode
open lcc.xcodeproj

# 3. Run (⌘ + R)
```

**That's it!** See [`QUICKSTART.md`](QUICKSTART.md) for more details.

## For Developers

### Run Tests
```bash
fastlane test
```

### Deploy to TestFlight
```bash
fastlane beta
```
Requires Apple Developer account. See [`DEPLOYMENT.md`](DEPLOYMENT.md).

### File Structure

```
lcc/
├── Config/Environment.swift      # Configuration (API URLs, timeouts)
├── Services/                      # Backend services
│   ├── MetricsService.swift      # Grafana metrics
│   └── NetworkMonitor.swift      # Connection monitoring  
├── Utilities/Logger.swift         # Structured logging
├── APIService.swift               # Fetches camera feeds
├── ImagePreloader.swift           # Image caching
└── Views/                         # SwiftUI UI
```

See [`PROJECT_STRUCTURE.md`](PROJECT_STRUCTURE.md) for complete layout.

## Architecture

- **Language**: Swift 5.9
- **UI**: SwiftUI (MVVM pattern)
- **Networking**: URLSession + Combine
- **Caching**: Custom with LRU eviction
- **Logging**: OSLog (structured)
- **Metrics**: Custom service (Grafana-ready)

## Configuration

Edit `.env.local`:

```bash
LCC_API_BASE_URL=https://lcc.live
GRAFANA_METRICS_URL=https://lcc.live/api/metrics
DEBUG_LOGGING=true
```

## Testing

| Suite | Coverage |
|-------|----------|
| ImagePreloader | Caching, memory, loading |
| APIService | Network, JSON parsing |
| MediaItem | URL detection (image/video) |
| Environment | Configuration validation |

## Monitoring

Metrics automatically sent to your Grafana instance:
- API response times
- Image load times  
- User interactions
- Errors & warnings

**Privacy**: No PII collected, anonymous only.

## Deployment

```bash
# Push to main → auto-deploys via GitHub Actions
git push origin main

# Or manually
fastlane beta
```

See [`DEPLOYMENT.md`](DEPLOYMENT.md) for complete guide.

## Documentation

| Doc | Purpose |
|-----|---------|
| [`QUICKSTART.md`](QUICKSTART.md) | ⭐ **Start here** (5 min) |
| [`PROJECT_STRUCTURE.md`](PROJECT_STRUCTURE.md) | Visual file tree & architecture |
| [`DEPLOYMENT.md`](DEPLOYMENT.md) | TestFlight & App Store guide |
| [`CHANGELOG.md`](CHANGELOG.md) | Version history |

## Contributing

1. Fork the repo
2. Create feature branch (`git checkout -b feature/amazing`)
3. Write tests for new features
4. Submit pull request

## Requirements

- iOS 26.0+
- Xcode 15.2+
- Ruby 3.2+ (for Fastlane)

## License

[Your license here]

## Contact

- **Website**: [lcc.live](https://lcc.live)
- **Issues**: [GitHub Issues](https://github.com/your-username/lcc.live.ios/issues)

---

**Made with ❤️ for the Utah outdoor community**
