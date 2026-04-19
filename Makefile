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

sync-brick: ## Deploy rex (into Debian chroot), termsp (Knulli), fonts, and launch scripts to TrimUI Brick
	@echo "Syncing to TrimUI Brick..."
	ssh root@192.168.1.213 'mkdir -p /userdata/system/apps/rex/fonts /userdata/system/apps/rex/lib /userdata/roms/ports /userdata/debian/root/bin'
	scp bin/rex-brick root@192.168.1.213:/userdata/debian/root/bin/rex
	scp bin/termsp root@192.168.1.213:/userdata/system/apps/rex/termsp
	scp ext/TermSP/TermSP/libs/libsdlfox.so ext/TermSP/TermSP/libs/libvterm.so root@192.168.1.213:/userdata/system/apps/rex/lib/
	ssh root@192.168.1.213 'ln -sf libvterm.so /userdata/system/apps/rex/lib/libvterm.so.0'
	scp ext/fonts/*.ttf root@192.168.1.213:/userdata/system/apps/rex/fonts/
	scp ext/brick/Rex.sh root@192.168.1.213:/userdata/roms/tools/Rex.sh
	scp ext/brick/start-rex.sh ext/brick/stop-rex.sh ext/brick/debian-rex.sh ext/brick/debian-sshd.sh root@192.168.1.213:/userdata/system/apps/rex/
	scp config.yaml.brick.example root@192.168.1.213:/userdata/system/apps/rex/config.yaml.brick.example
	scp config/dot/tmux.conf root@192.168.1.213:/userdata/debian/root/.tmux.conf
	ssh root@192.168.1.213 'mkdir -p /userdata/debian/root/.config/rex && [ -f /userdata/debian/root/.config/rex/config.yaml ] || cp /userdata/system/apps/rex/config.yaml.brick.example /userdata/debian/root/.config/rex/config.yaml'
	ssh root@192.168.1.213 'chmod +x /userdata/debian/root/bin/rex /userdata/system/apps/rex/termsp /userdata/roms/tools/Rex.sh /userdata/system/apps/rex/start-rex.sh /userdata/system/apps/rex/stop-rex.sh /userdata/system/apps/rex/debian-rex.sh /userdata/system/apps/rex/debian-sshd.sh'

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

setup-debian: ## (Re)install Debian packages, sherpa libs, and claude-code inside the chroot (idempotent)
	@echo "Installing sherpa/onnx libs into chroot /usr/local/lib and running ldconfig..."
	ssh root@192.168.1.213 'set -eu; \
		for lib in libsherpa-onnx-c-api.so libonnxruntime.so; do \
			[ -f /userdata/system/apps/rex/lib/$$lib ] && cp -u /userdata/system/apps/rex/lib/$$lib /userdata/debian/usr/local/lib/ || echo "warn: $$lib not found on host, skipping"; \
		done; \
		/userdata/system/apps/rex/debian-enter.sh "ldconfig"'
	@echo "Installing apt packages + claude-code inside chroot..."
	ssh root@192.168.1.213 '/userdata/system/apps/rex/debian-enter.sh "apt update -qq && apt install -y -qq libsdl2-2.0-0 openssh-server nodejs npm && npm install -g @anthropic-ai/claude-code 2>&1 | tail -5 && which claude && claude --version"'
	@echo "Configuring sshd in chroot (port 2222, pubkey auth)..."
	ssh root@192.168.1.213 '/userdata/system/apps/rex/debian-enter.sh "\
		mkdir -p /etc/ssh/sshd_config.d && \
		echo Port 2222 > /etc/ssh/sshd_config.d/rex.conf && \
		echo PermitRootLogin prohibit-password >> /etc/ssh/sshd_config.d/rex.conf && \
		mkdir -p /root/.ssh && chmod 700 /root/.ssh && \
		cp /userdata/system/.ssh/authorized_keys /root/.ssh/authorized_keys && \
		chmod 600 /root/.ssh/authorized_keys"'
	@echo "Done. To start sshd on boot, add this line to /userdata/system/custom.sh:"
	@echo "  /userdata/system/apps/rex/debian-sshd.sh"
	@echo "Connect with: ssh -p 2222 root@192.168.1.213"

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
