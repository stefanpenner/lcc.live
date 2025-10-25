# LCC Live

Live webcam feed for Little Cottonwood Canyon (LCC) and Big Cottonwood Canyon (BCC). Visit [lcc.live](https://lcc.live/).

## Quick Start

```bash
./b run        # Build and run
./b test       # Run tests
bazel run //:lcc-live -- --help
```

## Architecture

- **Go backend**: Single binary (~7.8MB), in-memory cache, background sync
- **Vanilla JS frontend**: No frameworks, ES modules
- **Build**: Bazel with rules_oci
- **Deploy**: Fly.io

## Development

```bash
# Run locally
bazel run //:lcc-live

# Add camera: edit data.json
# Modify UI: edit templates/ or static/
# Backend: edit server/ or store/
```

## Configuration

- `PORT` - HTTP port (default: 3000)
- `SYNC_INTERVAL` - Image refresh (default: 3s)
- `DEV_MODE=1` - Hot reload from disk

## Docs

See `doc/` for:
- `DEVELOPMENT.md` - Local setup
- `DEPLOYMENT.md` - Fly.io deployment
- `BAZEL.md` - Build system

