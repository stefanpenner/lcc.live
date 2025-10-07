## LCC Live

[![CI](https://github.com/stefanpenner/lcc.live/actions/workflows/ci.yml/badge.svg)](https://github.com/stefanpenner/lcc.live/actions/workflows/ci.yml)
[![Fuzz Testing](https://github.com/stefanpenner/lcc.live/actions/workflows/fuzz.yml/badge.svg)](https://github.com/stefanpenner/lcc.live/actions/workflows/fuzz.yml)

Fast, single-binary Go service that serves live canyon webcams for Little and Big Cottonwood Canyons. Visit [lcc.live](https://lcc.live/).

### Features
- **Single binary, tiny image**: ships as one executable; tiny Alpine container (~7.8MB total)
- **In-memory serving**: images via a custom store; static assets via embedded FS
- **Efficient fetch loop**: background sync keeps images fresh
- **Echo-powered HTTP**: simple, fast web + API server
- **Prometheus metrics**: exported at `/_/metrics`
- **Bazel build**: reproducible builds with Bzlmod

### Quick start
```bash
# Run the server
bazel run //:lcc-live

# Build and test
bazel build //:lcc-live
bazel test //...
```
Visit `http://localhost:3000`.

### Configuration
- **PORT**: HTTP port (default: `3000`)
- **SYNC_INTERVAL**: image refresh interval, Go duration (default: `3s`)

### Operations
- **Prometheus metrics**: `/_/metrics`
- **Cloudflare purge**:
  ```bash
  CLOUDFLARE_ZONE_ID=... CLOUDFLARE_API_TOKEN=... \
    bazel run //:lcc-live -- purge-cache
  ```

---

PRs welcome.
