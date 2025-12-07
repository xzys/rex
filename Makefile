.PHONY: docker-build build clean


docker-build: ## Build for R36S using Docker (SDL mode)
	docker build -t r36s-keyboard-builder src/
	docker run --rm --platform linux/amd64 \
		-v go-mod-cache:/go/pkg/mod \
		-v go-build-cache:/root/.cache/go-build \
		-v $$(pwd)/src:/app \
		-w /app \
		r36s-keyboard-builder \
		go build -tags sdl -o ../tr3x

build-local: ## Build for local machine with keyboard input
	cd src && go build -tags keyboard -o ../tr3x-local

debug:
	cd src && dlv debug . -- --server

build-sdl: ## Build for machine with SDL input
	cd src && go build -tags sdl -o ../tr3x

rebuild-r36s: docker-build
	scp tr3x r36s:bin/
