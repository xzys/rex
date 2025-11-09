.PHONY: docker-build build clean


docker-build: ## Build for R36S using Docker (SDL mode)
	docker build -t r36s-keyboard-builder .
	docker create --name r36s-keyboard-builder r36s-keyboard-builder
	docker cp r36s-keyboard-builder:/app/r36s-keyboard ./r36s-keyboard
	docker rm r36s-keyboard-builder

build-local: ## Build for local machine with keyboard input
	go build -tags keyboard -o local-keyboard

build-sdl: ## Build for machine with SDL input
	go build -tags sdl -o r36s-keyboard

build-server: ## Build the server binary
	go build -o tr3x-server ./server

rebuild-r36s: docker-build
	scp r36s-keyboard r36s:bin/
