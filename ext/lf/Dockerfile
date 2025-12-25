FROM --platform=linux/amd64 debian:buster

# --- configure archive repositories (Buster is EOL) -------------------------
RUN sed -i 's/deb.debian.org/archive.debian.org/g' /etc/apt/sources.list && \
    sed -i 's|security.debian.org/debian-security|archive.debian.org/debian-security|g' /etc/apt/sources.list && \
    sed -i '/stretch-updates/d' /etc/apt/sources.list

# --- packages ---------------------------------------------------------------
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates curl xz-utils git \
        && rm -rf /var/lib/apt/lists/*

ARG GO_VERSION=1.24.1

# --- Go tool-chain (linux-amd64 tarball) ------------------------------------
RUN curl -fsSL https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz \
      | tar -C /usr/local -xz
ENV PATH=/usr/local/go/bin:$PATH

# --- cross-compile env -------------------------------------------------------
ENV GOOS=linux GOARCH=arm64 CGO_ENABLED=0

WORKDIR /app