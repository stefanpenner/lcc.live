# Development Guide

## Code Quality & Testing

### Gazelle BUILD File Verification

To ensure all BUILD files are up to date and correctly generated:

```bash
bazel run //:gazelle_check
```

This will check if your BUILD files match what Gazelle would generate. If there are differences, run:

```bash
bazel run //:gazelle
```

### Go Linting with Nogo

The project uses Bazel's `nogo` integration for Go static analysis. Linting runs automatically during builds and tests. The linter configuration is in `nogo_config.json`.

To explicitly verify linting:

```bash
bazel build //... --aspects=@rules_go//go:def.bzl%go_vet_aspect --output_groups=vet
```

The nogo linter runs standard Go vet analyzers including:
- Type checking
- Unreachable code detection  
- Printf formatting verification
- Struct tag validation
- And more...

### Running Tests

Run all tests:

```bash
bazel test //...
```

Run specific package tests:

```bash
bazel test //server:all
bazel test //store:all
```

### Building

Build the binary:

```bash
bazel build //:lcc-live
```

Build and load the Docker image:

```bash
bazel run //:image_load
```

## Continuous Integration

### CI Workflow

The CI workflow (`.github/workflows/ci.yaml`) runs on all pushes and pull requests:

1. **Verifies BUILD files** are up to date with `bazel run //:gazelle_check`
2. **Runs all tests** with `bazel test //...`
3. **Builds the binary** to ensure compilation succeeds
4. **Verifies linting** with nogo

### Automated Dependency Updates

The dependency update workflow (`.github/workflows/dependency_update.yaml`) runs weekly and:

1. **Checks for Go module updates** using `go get -u ./...`
2. **Creates or updates a PR** with dependency changes
3. **Auto-merges the PR** when all CI checks pass

#### Manual Trigger

You can manually trigger a dependency update check:

```bash
gh workflow run dependency_update.yaml
```

#### Auto-Merge Behavior

The dependency update PR will automatically merge when:
- ✅ All CI tests pass
- ✅ The PR is up to date with the target branch
- ✅ No conflicts exist

To enable auto-merge in your repository:
1. Go to repository Settings → General → Pull Requests
2. Check "Allow auto-merge"
3. Go to Settings → Branches
4. Add branch protection rules requiring status checks

## Local Development Setup

### Prerequisites

- Go 1.23.3 or later
- Bazel (via Bazelisk)
- Docker (for container builds)

### Quick Start

```bash
# Run tests
bazel test //...

# Build and run locally
bazel run //:lcc-live

# Build Docker image
bazel run //:image_load

# Run the container
docker run -p 3000:3000 lcc.live:latest

# Deploy to Fly.io
./b deploy
# or: bazel run //:deploy

# Deploy to local Docker (builds and runs automatically)
./b deploy local
# or: bazel run //:deploy -- local
# Then test: curl http://localhost:3000/healthcheck
```

### Development Workflow

1. Make code changes
2. Run `bazel run //:gazelle` to update BUILD files
3. Run `bazel test //...` to verify tests pass
4. Run `bazel run //:gazelle_check` to verify BUILD files are correct
5. Commit changes

## Deployment

### Deploy to Fly.io (Production)

Deploy the optimized image to Fly.io:

```bash
./b deploy
# or: bazel run //:deploy
```

This will:
1. Build the OCI image with `--config=opt`
2. Load the image into Docker
3. Deploy to Fly.io using `fly deploy --local-only`
4. Show the deployment status

### Deploy to Local Docker (Testing)

Build, load, and run the image locally without deploying to Fly.io:

```bash
./b deploy local
# or: bazel run //:deploy -- local
```

This will:
1. Build the OCI image with `--config=opt`
2. Load the image into Docker as `lcc.live:latest`
3. Stop and remove any existing `lcc-live` containers
4. Start a new container named `lcc-live` on port 3000

The container runs automatically. Test it:

```bash
# Check health
curl http://localhost:3000/healthcheck

# Check version
curl http://localhost:3000/_/version

# View logs
docker logs -f lcc-live

# Stop the container
docker stop lcc-live
```

## Troubleshooting

### BUILD files out of sync

If you get errors about missing dependencies or BUILD files:

```bash
bazel run //:gazelle
```

### Stale Bazel cache

If you encounter weird build issues:

```bash
bazel clean --expunge
```

### Dependency issues

If dependencies seem out of sync:

```bash
go mod tidy
bazel run //:gazelle
```

