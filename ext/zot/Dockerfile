FROM --platform=linux/amd64 debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates curl xz-utils \
        && rm -rf /var/lib/apt/lists/*

WORKDIR /app
