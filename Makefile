.PHONY: docker-build build clean


docker-build: ## Build for R36S using Docker (SDL mode)
	docker build -t r36s-keyboard-builder .
	docker run --rm --platform linux/amd64 \
		-v go-mod-cache:/go/pkg/mod \
		-v go-build-cache:/root/.cache/go-build \
		-v $$(pwd):/build \
		-w /build \
		r36s-keyboard-builder \
		go build -tags sdl -o r36s-keyboard

build-local: ## Build for local machine with keyboard input
	go build -tags keyboard -o local-keyboard

debug:
	dlv debug . -- --server

build-sdl: ## Build for machine with SDL input
	go build -tags sdl -o r36s-keyboard

build-server: ## Build the server binary
	go build -o tr3x-server ./server

rebuild-r36s: docker-build
	scp r36s-keyboard r36s:bin/
