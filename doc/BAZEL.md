# Bazel Build System

This project uses [Bazel](https://bazel.build/) 8.x with [Bzlmod](https://bazel.build/external/module) for building.

## Prerequisites

Bazel will be automatically downloaded and installed at version 8.0.0 (specified in `.bazelversion`).

## Common Commands

### Building

```bash
# Build the main binary
bazel build //:lcc-live

# Build everything
bazel build //...

# Build with optimizations (for production)
bazel build --config=opt //:lcc-live
```

### Testing

```bash
# Run all tests
bazel test //...

# Run specific package tests
bazel test //store:store_test
bazel test //server:server_test

# Run tests with verbose output
bazel test //... --test_output=all
```

### Running

```bash
# Run the binary directly
bazel run //:lcc-live

# Or run the built binary
./bazel-bin/lcc-live_/lcc-live
```

### Cleaning

```bash
# Clean build outputs
bazel clean

# Deep clean (removes all cached artifacts)
bazel clean --expunge
```

## Gazelle - BUILD File Generation

[Gazelle](https://github.com/bazelbuild/bazel-gazelle) automatically generates and maintains BUILD.bazel files for Go packages.

### Update BUILD files after code changes

```bash
# Regenerate all BUILD files
bazel run //:gazelle

# Update after changing go.mod dependencies
bazel run //:gazelle-update-repos
bazel run //:gazelle
```

### Gazelle directives

The root `BUILD.bazel` contains Gazelle configuration:
- `# gazelle:prefix github.com/stefanpenner/lcc-live` - Go module import path
- `# gazelle:exclude tmp` - Exclude directories from Gazelle scanning

## Module System (Bzlmod)

This project uses Bazel's new module system (Bzlmod) via `MODULE.bazel`:

- **Go SDK**: Version 1.23.3 (matches `go.mod`)
- **rules_go**: v0.50.1
- **gazelle**: v0.39.1

All Go dependencies are automatically loaded from `go.mod` via the `go_deps` extension.

## Configuration

### .bazelrc

The `.bazelrc` file contains build configurations:
- `--config=fast` - Fast builds for development
- `--config=opt` - Optimized builds for production  
- `--config=debug` - Debug builds with symbols

### Build Modes

```bash
# Development (fast, unoptimized)
bazel build //:lcc-live

# Production (optimized, stripped)
bazel build --config=opt //:lcc-live

# Debug (with debug symbols)
bazel build --config=debug //:lcc-live
```

## IDE Integration

### VS Code

Install the [Bazel extension](https://marketplace.visualstudio.com/items?itemName=BazelBuild.vscode-bazel).

### GoLand / IntelliJ

Install the [Bazel plugin](https://plugins.jetbrains.com/plugin/8609-bazel) and configure it to use the project.

## Troubleshooting

### Cache issues

If you encounter strange build errors, try:
```bash
bazel clean
bazel build //...
```

### Dependencies not found

Make sure `go.mod` is up to date, then run:
```bash
bazel run //:gazelle-update-repos
bazel run //:gazelle
```

### Bazel version mismatch

The project requires Bazel 8.x. If you have a different version installed globally, Bazelisk will automatically download and use version 8.0.0 as specified in `.bazelversion`.

## Performance Tips

1. **Use remote cache** - Set up a remote cache for faster builds across machines
2. **Parallel builds** - Bazel automatically parallelizes builds
3. **Incremental builds** - Only rebuilds what changed
4. **Sandbox cleanup** - The `.bazelrc` enables sandbox directory reuse for faster builds

## Additional Resources

- [Bazel Documentation](https://bazel.build/)
- [rules_go Documentation](https://github.com/bazelbuild/rules_go)
- [Gazelle Documentation](https://github.com/bazelbuild/bazel-gazelle)
- [Bzlmod Guide](https://bazel.build/external/module)

