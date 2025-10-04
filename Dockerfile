FROM golang:1.23-alpine as builder

# Install build dependencies
RUN apk add --no-cache \
    bash \
    ca-certificates \
    curl \
    git \
    gcc \
    musl-dev

# Install Bazelisk (respects .bazelversion file)
RUN curl -Lo /usr/local/bin/bazel https://github.com/bazelbuild/bazelisk/releases/download/v1.20.0/bazelisk-linux-amd64 && \
    chmod +x /usr/local/bin/bazel

WORKDIR /usr/src/app

# Copy all source files needed for Bazel build
COPY . .

# Build with Bazel (production build with stamping)
RUN bazel build --config=opt //:lcc-live

# Extract the binary from Bazel's output
RUN cp bazel-bin/lcc-live_/lcc-live /usr/local/bin/lcc.live

# Final stage - minimal runtime image
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates curl

COPY --from=builder /usr/local/bin/lcc.live /usr/local/bin/lcc.live
COPY purge-cache.sh /usr/local/bin/purge-cache.sh
RUN chmod +x /usr/local/bin/purge-cache.sh

CMD ["lcc.live"]
