.PHONY: build clean docker-build

# Default target
all: build

# Build using Docker
docker-build:
	docker build -t r36s-keyboard-builder .
	docker create --name temp-container r36s-keyboard-builder
	docker cp temp-container:/app/r36s-keyboard ./r36s-keyboard
	docker rm temp-container

# Build locally (requires cross-compilation tools)
build:
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=1 CC=arm-linux-gnueabihf-gcc go build -o r36s-keyboard

# Clean build artifacts
clean:
	rm -f r36s-keyboard
	docker rm -f temp-container 2>/dev/null || true
	docker rmi r36s-keyboard-builder 2>/dev/null || true