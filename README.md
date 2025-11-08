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
# Run locally (uses Doppler for secrets injection)
./b run

# Or run directly with Bazel (requires NEON_DATABASE_URL env var)
bazel run //:lcc-live

# Add camera: edit seed.json (then run seed-neon to update database)
# Modify UI: edit templates/ or static/
# Backend: edit server/ or store/
```

**Note**: The `./b run` command automatically uses Doppler to inject secrets (including `NEON_DATABASE_URL`). Make sure you've run `doppler setup --project lcc-live --config dev` first.

## Configuration

- `PORT` - HTTP port (default: 3000)
- `SYNC_INTERVAL` - Image refresh (default: 3s)
- `DEV_MODE=1` - Hot reload from disk

## Docs

See `doc/` for:
- `DEVELOPMENT.md` - Local setup
- `DEPLOYMENT.md` - Fly.io deployment
- `BAZEL.md` - Build system

