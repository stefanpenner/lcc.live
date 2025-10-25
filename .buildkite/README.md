# Buildkite Pipeline

This directory contains the Buildkite pipeline configuration for CI/CD.

## Pipeline Configuration

The `pipeline.yml` file defines the CI workflow that runs on Buildkite. It mirrors the GitHub Actions workflow (`.github/workflows/ci.yaml`) for consistency.

### What It Does

1. **Setup**: Installs Bazelisk and configures Bazel caches
2. **BuildBuddy**: Configures remote cache if `BUILDBUDDY_API_KEY` is set
3. **Docker**: Detects if Docker is available for integration tests
4. **Tests**: Runs all unit and integration tests
5. **Cleanup**: Removes temporary configuration files

### Environment Variables

- `BUILDBUDDY_API_KEY` - Optional BuildBuddy API key for remote caching

### Test Filters

If Docker is not available, the pipeline automatically excludes integration and local tests:
```bash
bazel test --test_tag_filters=-integration,-local //...
```

### Monitoring

Build logs and test results are available in the Buildkite UI. With BuildBuddy enabled, additional metrics are available at https://app.buildbuddy.io.
