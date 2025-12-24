.PHONY: build-local debug install-system-files install-dotfiles install-bin install-systemd

build-local: ## Build for local machine with keyboard input
	cd src && go build -tags keyboard -o ../bin/rex-local

debug:
	cd src && dlv debug . -- --server

docker-build-r36s: ## Build for R36S using Docker
	docker build -f src/Dockerfile -t r36s-keyboard-builder src/
	docker run --rm --platform linux/amd64 \
		-v go-mod-cache:/go/pkg/mod \
		-v go-build-cache:/root/.cache/go-build \
		-v $$(pwd)/src:/app \
		-v $$(pwd)/bin:/output \
		-w /app \
		r36s-keyboard-builder \
		go build -tags sdl -o /output/rex

sync-r36s:
	rsync bin/rex r36s:bin/
	ssh r36s 'mkdir -p ~/.config/rex'
	rsync config.yaml r36s:~/.config/rex/

docker-build-ext-%: ## Build program from ext/
	docker build -f ext/$*/Dockerfile -t r36s-$*-builder ext/$*
	docker run --rm --platform linux/amd64 \
		-v go-mod-cache:/go/pkg/mod \
		-v go-build-cache:/root/.cache/go-build \
		-e GOPATH=/root/go \
		-v $$(pwd)/bin:/output \
		-v $$(pwd)/ext/$*:/app \
		-w /app \
		r36s-$*-builder \
		/app/build.sh


install-system-files: install-dotfiles install-bin install-systemd ## Install all system files to r36s

install-dotfiles:
	@echo "Syncing dotfiles..."
	for f in config/dot/*; do \
		rsync "$$f" r36s:~/.$$(basename "$$f"); \
	done

install-bin:
	@echo "Syncing bin scripts..."
	ssh r36s 'mkdir -p ~/bin'
	rsync -r config/bin/ r36s:bin/
	ssh r36s 'chmod +x ~/bin/*.sh'

install-systemd:
	@echo "Syncing systemd services..."
	ssh r36s 'mkdir -p /tmp/systemd'
	rsync -r config/systemd/ r36s:/tmp/systemd/
	ssh r36s 'for f in /tmp/systemd/*.service; do \
		sudo mv $$f /etc/systemd/system/; \
	done && \
	sudo systemctl daemon-reload; \
	rm -rf /tmp/systemd'
