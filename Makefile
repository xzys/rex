.PHONY: docker-build build clean help

# Default target
.DEFAULT_GOAL := help

# Build targets
docker-build: ## Build for R36S using Docker (SDL mode)
	docker build -t r36s-keyboard-builder .
	docker create --name r36s-keyboard-builder r36s-keyboard-builder
	docker cp r36s-keyboard-builder:/app/r36s-keyboard ./r36s-keyboard
	docker rm r36s-keyboard-builder

build-keyboard: ## Build for local machine with keyboard input
	go build -tags keyboard -o local-keyboard

build-sdl: ## Build for local machine with SDL input
	go build -tags sdl -o r36s-keyboard

rebuild-r36s: docker-build
	scp r36s-keyboard r36s:bin/
