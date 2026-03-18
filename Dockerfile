# syntax=docker/dockerfile:1

FROM alpine:3.21

# Metadata
LABEL maintainer="kula app GmbH <opensource@kula.app>"
LABEL description="Container for ship CLI"

# ARG for platform detection
ARG TARGETARCH

# Copy the appropriate binary based on target architecture
COPY dist/ship-linux-${TARGETARCH} /tmp/ship

# Install binary to PATH
RUN install \
    -o root \
    -g root \
    -m 0755 \
    /tmp/ship /usr/local/bin/ship && \
    rm -f /tmp/ship

# Smoke test
RUN set -x && \
    ship --version

# Set environment variables
ENV TZ=UTC
