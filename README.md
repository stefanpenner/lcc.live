## LCC Live

[![CI](https://github.com/stefanpenner/lcc.live/actions/workflows/ci.yml/badge.svg)](https://github.com/stefanpenner/lcc.live/actions/workflows/ci.yml)
[![Fuzz Testing](https://github.com/stefanpenner/lcc.live/actions/workflows/fuzz.yml/badge.svg)](https://github.com/stefanpenner/lcc.live/actions/workflows/fuzz.yml)

Fast, single-binary Go service serving live canyon webcams for Little and Big Cottonwood Canyons. Visit [lcc.live](https://lcc.live/).

### Features

**Backend (Go)**
- Single binary, tiny container (~7.8MB Alpine image)
- In-memory image store with ETag-based caching
- Background sync keeps images fresh (3s default)
- Embedded static assets and templates
- Prometheus metrics at `/_/metrics`

**Frontend (Vanilla JS)**
- ETag-aware image auto-refresh (no flicker)
- Fullscreen viewer with keyboard navigation
- Touch gestures for mobile (swipe, pinch)
- Network-adaptive polling (respects connection speed)
- No frameworks, just 460 lines of modern ES modules

### Quick Start

```bash
# Simple helper script
./b run              # Build and run server
./b test             # Run all tests

# Or use Bazel directly
bazel run //:lcc-live
bazel test //...
```

Visit `http://localhost:3000`

### Project Structure

```
├── main.go              # Entry point, config
├── server/              # HTTP handlers, routes
├── store/               # Image cache, data models
├── static/              # CSS, JavaScript
│   └── script.mjs       # Frontend (ES modules)
├── templates/           # Go HTML templates
├── data.json            # Camera definitions
└── doc/                 # Architecture docs
```

### Configuration

Environment variables:
- `PORT` - HTTP port (default: `3000`)
- `SYNC_INTERVAL` - Image refresh, Go duration (default: `3s`)

### Frontend Architecture

**No Build Step Required** - Vanilla JavaScript ES modules served directly.

Key features:
- **Double buffering**: Images fully decoded before swap (zero flicker)
- **Scroll-aware**: Pauses updates during active scrolling
- **iOS Safari optimized**: Avoids `content-visibility` and async decoding
- **Memory efficient**: Blob URL tracking with proper cleanup

See `static/script.mjs` for implementation.

### Contributing

**Prerequisites:**
- Go 1.21+
- Bazel 7+ (or use `./b` script)

**Making Changes:**
1. Edit code (Go, templates, or static files)
2. Run `./b test` to verify
3. Test locally with `./b run`
4. Submit PR

**Common Tasks:**
- Add camera: Edit `data.json`
- Modify UI: Edit `templates/*.html.tmpl` or `static/*`
- Backend logic: Edit `server/*.go` or `store/*.go`

### Operations

**Metrics:** `/_/metrics` (Prometheus format)

**Cache Purge:**
```bash
CLOUDFLARE_ZONE_ID=... CLOUDFLARE_API_TOKEN=... \
  bazel run //:lcc-live -- purge-cache
```

**Deployment:** See `doc/DEPLOYMENT.md`

---

PRs welcome! Keep it simple.

