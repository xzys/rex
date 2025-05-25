FROM golang:1.24

# Enable armhf architecture and install cross tools and ARM SDL2 dev
RUN dpkg --add-architecture armhf && \
    apt-get update && \
    apt-get install -y \
      gcc-arm-linux-gnueabihf \
      g++-arm-linux-gnueabihf \
      pkg-config \
      libsdl2-dev:armhf \
      libstdc++-12-dev:armhf \
      && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Set up cross-compilation environment
ENV GOOS=linux \
    GOARCH=arm \
    GOARM=7 \
    CGO_ENABLED=1 \
    CC=arm-linux-gnueabihf-gcc \
    CXX=arm-linux-gnueabihf-g++ \
    PKG_CONFIG_PATH=/usr/lib/arm-linux-gnueabihf/pkgconfig \
    CGO_CFLAGS='-I/usr/include/SDL2' \
    CGO_LDFLAGS='-L/usr/lib/arm-linux-gnueabihf'

# Build the application
RUN go build -v -o r36s-keyboard

# The output binary will be in the /app directory 