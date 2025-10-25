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

- `BUILDBUDDY_API_KEY` - BuildBuddy API key for remote caching (provided via Buildkite secrets)

### Test Filters

If Docker is not available, the pipeline automatically excludes integration and local tests:
```bash
bazel test --test_tag_filters=-integration,-local //...
```

### Running as Non-Root User

The test script automatically handles running as a non-root user when needed:
- **Buildkite defaults to root**: Buildkite agents run as the root user by default
- **rules_python restriction**: The hermetic Python interpreter (required by `rules_pkg`) refuses to run as root
- **Automatic user creation**: The script creates a `buildkite` user and re-runs itself as that user
- **All tests run**: This allows the container test to run in Buildkite, matching GitHub Actions behavior

This ensures both Buildkite and GitHub Actions run the same tests.

### Docker Support

The pipeline automatically installs and configures Docker if not already available:
- **Automatic installation**: Docker CE is installed from official Docker repositories
- **Rootless operation**: Current user is added to docker group for non-root operation
- **Container tests**: Full integration tests including `//:container_test` can run
- **Startup handling**: Docker daemon is automatically started and verified before tests

### Monitoring

Build logs and test results are available in the Buildkite UI. With BuildBuddy enabled, additional metrics are available at https://app.buildbuddy.io.
