.PHONY: build-local debug install-system-files install-dotfiles install-bin install-systemd

build-local: ## Build for local machine with keyboard input
	cd src && go build -tags keyboard -o ../rex-local

debug:
	cd src && dlv debug . -- --server

build-docker-r36s: ## Build for R36S using Docker
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
	@echo "Syncing rex binary and scripts to r36s..."
	rsync bin/rex r36s:bin/
	ssh r36s 'mkdir -p ~/.config/rex'
	rsync config.yaml r36s:~/.config/rex/

build-docker-brick: ## Build rex for TrimUI Brick (cross-compile ARM64, SDL+brick tags)
	docker build -f src/Dockerfile -t brick-rex-builder src/
	docker run --rm --platform linux/amd64 \
		-v go-mod-cache:/go/pkg/mod \
		-v go-build-cache:/root/.cache/go-build \
		-v $$(pwd)/src:/app \
		-v $$(pwd)/bin:/output \
		-w /app \
		brick-rex-builder \
		go build -tags "sdl brick sherpa" -o /output/rex-brick

sync-brick: ## Deploy rex, termsp, tmux, fonts, and launch script to TrimUI Brick
	@echo "Syncing to TrimUI Brick..."
	ssh root@192.168.1.213 'mkdir -p /userdata/system/apps/rex/fonts /userdata/system/apps/rex/lib /userdata/roms/ports'
	scp bin/rex-brick root@192.168.1.213:/userdata/system/apps/rex/rex
	scp bin/termsp root@192.168.1.213:/userdata/system/apps/rex/termsp
	scp bin/tmux root@192.168.1.213:/userdata/system/apps/rex/tmux
	scp ext/TermSP/TermSP/libs/libsdlfox.so ext/TermSP/TermSP/libs/libvterm.so root@192.168.1.213:/userdata/system/apps/rex/lib/
	ssh root@192.168.1.213 'ln -sf libvterm.so /userdata/system/apps/rex/lib/libvterm.so.0'
	scp ext/fonts/*.ttf root@192.168.1.213:/userdata/system/apps/rex/fonts/
	scp ext/brick/Rex.sh root@192.168.1.213:/userdata/roms/tools/Rex.sh
	scp ext/brick/start-rex.sh ext/brick/stop-rex.sh ext/brick/debian-launch-rex.sh ext/brick/debian-rex.sh root@192.168.1.213:/userdata/system/apps/rex/
	scp config.yaml.brick.example root@192.168.1.213:/userdata/system/apps/rex/config.yaml.brick.example
	scp config/dot/tmux.conf root@192.168.1.213:/userdata/debian/root/.tmux.conf
	ssh root@192.168.1.213 'mkdir -p /userdata/debian/root/.config/rex && [ -f /userdata/debian/root/.config/rex/config.yaml ] || cp /userdata/system/apps/rex/config.yaml.brick.example /userdata/debian/root/.config/rex/config.yaml'
	ssh root@192.168.1.213 'chmod +x /userdata/system/apps/rex/rex /userdata/system/apps/rex/termsp /userdata/system/apps/rex/tmux /userdata/roms/tools/Rex.sh /userdata/system/apps/rex/start-rex.sh /userdata/system/apps/rex/stop-rex.sh /userdata/system/apps/rex/debian-launch-rex.sh /userdata/system/apps/rex/debian-rex.sh'

bootstrap-debian: ## Build Debian arm64 rootfs tarball for Brick chroot (bin/brick-debian.tar.gz)
	bash scripts/bootstrap-debian-brick.sh

sync-debian: ## Transfer Debian rootfs to Brick and extract to /userdata/debian
	@test -f bin/brick-debian.tar.gz || { echo "bin/brick-debian.tar.gz missing; run 'make bootstrap-debian' first"; exit 1; }
	@echo "Transferring rootfs tarball to Brick (may take a few minutes)..."
	scp bin/brick-debian.tar.gz root@192.168.1.213:/userdata/
	scp ext/brick/debian-enter.sh ext/brick/debian-cleanup.sh root@192.168.1.213:/userdata/system/apps/rex/
	ssh root@192.168.1.213 'set -e; \
		mkdir -p /userdata/debian; \
		[ -z "$$(ls -A /userdata/debian 2>/dev/null)" ] || { echo "/userdata/debian is not empty; refusing to extract"; exit 1; }; \
		tar xzf /userdata/brick-debian.tar.gz -C /userdata/debian; \
		rm /userdata/brick-debian.tar.gz; \
		chmod +x /userdata/system/apps/rex/debian-enter.sh /userdata/system/apps/rex/debian-cleanup.sh; \
		echo "Debian chroot ready."'
	$(MAKE) setup-debian

setup-debian: ## (Re)install standard Debian packages + claude-code inside the chroot (idempotent)
	ssh root@192.168.1.213 '/userdata/system/apps/rex/debian-enter.sh "apt update -qq && apt install -y -qq libsdl2-2.0-0 nodejs npm && npm install -g @anthropic-ai/claude-code 2>&1 | tail -5 && which claude && claude --version"'

build-docker-ext-%: ## Build program from ext/
	docker build -f ext/$*/Dockerfile -t r36s-$(shell echo $* | tr '[:upper:]' '[:lower:]')-builder ext/$*
	docker run --rm --platform linux/amd64 \
		-v go-mod-cache:/go/pkg/mod \
		-v go-build-cache:/root/.cache/go-build \
		-e GOPATH=/root/go \
		-v $$(pwd)/bin:/output \
		-v $$(pwd)/ext/$*:/app \
		-w /app \
		r36s-$(shell echo $* | tr '[:upper:]' '[:lower:]')-builder \
		/app/build.sh


install-system-files: install-dotfiles install-systemd install-opt ## Install all system files to r36s

install-opt:
	@echo "Syncing opt scripts..."
	rsync -r config/system/opt/ r36s:/opt/

install-dotfiles:
	@echo "Syncing dotfiles..."
	for f in config/dot/*; do \
		rsync "$$f" r36s:~/.$$(basename "$$f"); \
	done

install-systemd:
	@echo "Syncing systemd services..."
	ssh r36s 'mkdir -p /tmp/systemd'
	rsync -r config/systemd/ r36s:/tmp/systemd/
	ssh r36s 'for f in /tmp/systemd/*.service; do \
		sudo mv $$f /etc/systemd/system/; \
	done && \
	for d in /tmp/systemd/*.service.d; do \
		[ -d "$$d" ] && sudo cp -r "$$d" /etc/systemd/system/; \
	done && \
	sudo systemctl daemon-reload; \
	rm -rf /tmp/systemd'
