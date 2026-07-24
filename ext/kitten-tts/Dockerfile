# Build kitten-tts (KittenTTS CLI) for Debian Bookworm aarch64.
# Cross-compiles with the official onnxruntime v1.23.2 for glibc 2.36 compat.
#
# Usage:
#   docker buildx build --platform linux/amd64 --load -t kitten-tts-builder ext/kitten-tts/
#   docker run --rm --platform linux/amd64 \
#     -v $(pwd)/bin:/output -v $(pwd)/ext/kitten-tts:/app -w /app \
#     kitten-tts-builder /app/build.sh

FROM debian:bookworm-slim

RUN apt-get update -qq && \
    apt-get install -y -qq --no-install-recommends \
        curl ca-certificates build-essential pkg-config libssl-dev cmake git \
    && rm -rf /var/lib/apt/lists/*

# Install Rust
RUN curl --proto =https --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y -q

ENV PATH="/root/.cargo/bin:$PATH"
ENV ORT_VERSION=1.23.2
ENV ORT_LIB_LOCATION=/opt/onnxruntime

# Download official onnxruntime (glibc 2.27 compatible)
RUN mkdir -p /opt/onnxruntime && \
    curl -fsSL "https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}/onnxruntime-linux-aarch64-${ORT_VERSION}.tgz" \
        -o /tmp/ort.tgz && \
    tar xzf /tmp/ort.tgz -C /tmp && \
    cp /tmp/onnxruntime-linux-aarch64-${ORT_VERSION}/lib/libonnxruntime.so.${ORT_VERSION} /opt/onnxruntime/libonnxruntime.so && \
    rm -rf /tmp/ort.tgz /tmp/onnxruntime-linux-aarch64-${ORT_VERSION}

ENV ORT_PREFER_DYNAMIC_LINK=1

COPY build.sh /app/build.sh
RUN chmod +x /app/build.sh
