# GitHub Actions Workflows

This directory contains the CI/CD workflows for the lcc.live project.

## Workflows

### 1. CI Workflow (`ci.yml`)
**Triggers:** Push to `main`/`echo` branches, Pull Requests

Main build and test pipeline that runs on every push and PR:

- **Build and Test Job:**
  - Builds all Bazel targets (`bazel build //...`)
  - Runs all tests (`bazel test //...`)
  - Verifies the binary can be built
  - Uses Bazel caching for faster builds

- **Lint Job:**
  - Runs `golangci-lint` for code quality checks
  - Verifies Gazelle formatting is up-to-date
  - Ensures BUILD.bazel files are properly generated

**Cache Strategy:** Caches Bazel build artifacts and Bazelisk downloads to speed up subsequent runs.

### 2. Fuzz Testing Workflow (`fuzz.yml`)
**Triggers:** Manual dispatch, Weekly schedule (Sundays at 2 AM UTC), Push to `main` (Go files only)

Runs comprehensive fuzzing tests using the project's `fuzz-all.sh` script:

- Runs all fuzz tests in the codebase
- Default fuzz time: 30 seconds per test (configurable via workflow dispatch)
- Caches fuzz corpus between runs
- Uploads crash reports and corpus artifacts on failure
- Extended timeout (30 minutes) to handle long-running fuzz tests

**Manual Trigger:** You can manually trigger this workflow from the Actions tab and specify custom fuzz time (e.g., `5m`, `1h`).

### 3. Dependency Updates Workflow (`dependency-update.yml`)
**Triggers:** Weekly schedule (Mondays at 9 AM UTC), Manual dispatch

Automatically updates Go dependencies and creates a PR:

- Updates all Go dependencies (`go get -u ./...`)
- Regenerates Bazel repositories (`gazelle-update-repos`)
- Runs all tests to ensure compatibility
- Creates a PR with the updates automatically

**Note:** This workflow requires GitHub Actions to have write permissions to create PRs.

## Configuration Files

### `.golangci.yml`
Comprehensive linting configuration that includes:
- Error checking (errcheck, errorlint)
- Code simplification (gosimple, gocritic)
- Security checks (gosec)
- Style consistency (gofmt, goimports, revive, stylecheck)
- Performance checks
- Custom exclusions for test files and fuzz tests

## Badge Status

The main README includes status badges for:
- CI workflow status
- Fuzz testing status

## Local Development

To run the same checks locally:

```bash
# Build everything
bazel build //...

# Run all tests
bazel test //...

# Run linter
golangci-lint run

# Check Gazelle formatting
bazel run //:gazelle -- --mode=diff

# Run fuzz tests (5 seconds per test)
FUZZ_TIME=5s ./fuzz-all.sh

# Run fuzz tests (longer duration)
FUZZ_TIME=1m ./fuzz-all.sh
```

## Maintenance

- CI runs on every commit to ensure code quality
- Fuzz tests run weekly to catch edge cases
- Dependencies are checked weekly for updates
- All workflows use caching to minimize build times
- Workflow logs are retained for debugging

## Troubleshooting

### CI Failures
1. Check the build logs in the GitHub Actions tab
2. Run `bazel test //...` locally to reproduce
3. Ensure all BUILD.bazel files are up-to-date with `bazel run //:gazelle`

### Lint Failures
1. Run `golangci-lint run` locally
2. Fix reported issues
3. Run `bazel run //:gazelle` to update BUILD files if needed

### Fuzz Test Failures
1. Check uploaded artifacts for crash details
2. Download and examine the fuzz corpus
3. Run specific fuzz tests locally: `go test -fuzz=FuzzTestName -fuzztime=1m ./package`

## Future Improvements

Potential additions:
- Deployment workflow for production releases
- Performance benchmarking workflow
- Docker image building and publishing
- Code coverage reporting
- Security scanning (e.g., Snyk, Dependabot)

