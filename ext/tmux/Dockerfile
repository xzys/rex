FROM --platform=linux/amd64 debian:buster

# Buster is EOL — use archive mirrors
RUN sed -i 's/deb.debian.org/archive.debian.org/g' /etc/apt/sources.list && \
    sed -i 's|security.debian.org/debian-security|archive.debian.org/debian-security|g' /etc/apt/sources.list && \
    sed -i '/stretch-updates/d' /etc/apt/sources.list

RUN dpkg --add-architecture arm64 && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        build-essential gcc-aarch64-linux-gnu pkg-config curl ca-certificates \
        libncurses-dev:arm64 libevent-dev:arm64 \
        && rm -rf /var/lib/apt/lists/*

WORKDIR /app
