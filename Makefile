.PHONY: build-local debug install-system-files install-dotfiles install-bin install-systemd install-stt-sherpa install-stt-parakeet install-stt-models

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
	ssh brick-root 'mkdir -p /userdata/system/apps/rex/fonts /userdata/system/apps/rex/lib /userdata/roms/ports /userdata/debian/home/rex/bin /userdata/debian/home/rex/.config/rex'
	scp bin/rex-brick brick-root:/userdata/debian/home/rex/bin/rex
	scp bin/termsp brick-root:/userdata/system/apps/rex/termsp
	scp ext/TermSP/TermSP/libs/libsdlfox.so ext/TermSP/TermSP/libs/libvterm.so brick-root:/userdata/system/apps/rex/lib/
	ssh brick-root 'ln -sf libvterm.so /userdata/system/apps/rex/lib/libvterm.so.0'
	scp ext/fonts/*.ttf brick-root:/userdata/system/apps/rex/fonts/
	scp ext/brick/Rex.sh brick-root:/userdata/roms/tools/Rex.sh
	scp ext/brick/start-rex.sh ext/brick/stop-rex.sh ext/brick/debian-rex.sh ext/brick/debian-sshd.sh brick-root:/userdata/system/apps/rex/
	scp config.yaml.brick.example brick-root:/userdata/system/apps/rex/config.yaml.brick.example
	scp config/dot/tmux.conf brick-root:/userdata/debian/home/rex/.tmux.conf
	ssh brick-root '[ -f /userdata/debian/home/rex/.config/rex/config.yaml ] || cp /userdata/system/apps/rex/config.yaml.brick.example /userdata/debian/home/rex/.config/rex/config.yaml'
	ssh brick-root 'chmod +x /userdata/debian/home/rex/bin/rex /userdata/system/apps/rex/termsp /userdata/roms/tools/Rex.sh /userdata/system/apps/rex/start-rex.sh /userdata/system/apps/rex/stop-rex.sh /userdata/system/apps/rex/debian-rex.sh /userdata/system/apps/rex/debian-sshd.sh'
	@echo "Fixing ownership of /home/rex inside chroot (scp lands files as root)..."
	ssh brick-root '/userdata/system/apps/rex/debian-enter.sh --root "id rex >/dev/null 2>&1 && chown -R rex:rex /home/rex || echo warn: rex user not yet created, run make setup-debian"'

STT_SHERPA_URL = https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-nemo-ctc-en-conformer-small-2024-08-01.tar.bz2
STT_SHERPA_DIR = bin/sherpa-onnx-nemo-ctc-en-conformer-small-2024-08-01

STT_PARAKEET_URL = https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-nemo-parakeet_tdt_ctc_110m-en-36000-int8.tar.bz2
STT_PARAKEET_DIR = bin/sherpa-onnx-nemo-parakeet_tdt_ctc_110m-en-36000-int8

$(STT_SHERPA_DIR)/model.int8.onnx:
	@echo "Downloading NeMo Conformer-Small int8 (~45MB)..."
	curl -L -o bin/sherpa.tar.bz2 $(STT_SHERPA_URL)
	tar xjf bin/sherpa.tar.bz2 -C bin/
	rm bin/sherpa.tar.bz2

$(STT_PARAKEET_DIR)/model.int8.onnx:
	@echo "Downloading Parakeet-TDT-CTC 110M int8 (~120MB)..."
	curl -L -o bin/parakeet.tar.bz2 $(STT_PARAKEET_URL)
	tar xjf bin/parakeet.tar.bz2 -C bin/
	rm bin/parakeet.tar.bz2

install-stt-sherpa: $(STT_SHERPA_DIR)/model.int8.onnx ## Install NeMo Conformer-Small onto Brick (stt_mode: sherpa, default)
	@echo "Installing Conformer-Small model to /userdata/system/apps/rex/stt/..."
	ssh brick-root 'mkdir -p /userdata/system/apps/rex/stt'
	scp $(STT_SHERPA_DIR)/model.int8.onnx $(STT_SHERPA_DIR)/tokens.txt brick-root:/userdata/system/apps/rex/stt/
	ssh brick-root 'chmod -R a+rX /userdata/system/apps/rex/stt'

install-stt-parakeet: $(STT_PARAKEET_DIR)/model.int8.onnx ## Install Parakeet-TDT-CTC 110M onto Brick (stt_mode: parakeet)
	@echo "Installing Parakeet model to /userdata/system/apps/rex/stt-parakeet/..."
	ssh brick-root 'mkdir -p /userdata/system/apps/rex/stt-parakeet'
	scp $(STT_PARAKEET_DIR)/model.int8.onnx $(STT_PARAKEET_DIR)/tokens.txt brick-root:/userdata/system/apps/rex/stt-parakeet/
	ssh brick-root 'chmod -R a+rX /userdata/system/apps/rex/stt-parakeet'

install-stt-models: install-stt-sherpa install-stt-parakeet ## Install both STT models onto Brick so Settings app can switch dynamically
	@echo "Done. Switch between stt_mode: sherpa / parakeet in the rex Settings app."

bootstrap-debian: ## Build Debian arm64 rootfs tarball for Brick chroot (bin/brick-debian.tar.gz)
	bash scripts/bootstrap-debian-brick.sh

sync-debian: ## Transfer Debian rootfs to Brick and extract to /userdata/debian
	@test -f bin/brick-debian.tar.gz || { echo "bin/brick-debian.tar.gz missing; run 'make bootstrap-debian' first"; exit 1; }
	@echo "Transferring rootfs tarball to Brick (may take a few minutes)..."
	scp bin/brick-debian.tar.gz brick-root:/userdata/
	scp ext/brick/debian-enter.sh ext/brick/debian-cleanup.sh brick-root:/userdata/system/apps/rex/
	ssh brick-root 'set -e; \
		mkdir -p /userdata/debian; \
		[ -z "$$(ls -A /userdata/debian 2>/dev/null)" ] || { echo "/userdata/debian is not empty; refusing to extract"; exit 1; }; \
		tar xzf /userdata/brick-debian.tar.gz -C /userdata/debian; \
		rm /userdata/brick-debian.tar.gz; \
		chmod +x /userdata/system/apps/rex/debian-enter.sh /userdata/system/apps/rex/debian-cleanup.sh; \
		echo "Debian chroot ready."'
	$(MAKE) setup-debian

setup-debian: ## (Re)install Debian packages, sherpa libs, rex user, and claude-code inside the chroot (idempotent)
	@echo "Installing sherpa/onnx libs into chroot /usr/local/lib and running ldconfig..."
	ssh brick-root 'set -eu; \
		for lib in libsherpa-onnx-c-api.so libonnxruntime.so; do \
			[ -f /userdata/system/apps/rex/lib/$$lib ] && cp -u /userdata/system/apps/rex/lib/$$lib /userdata/debian/usr/local/lib/ || echo "warn: $$lib not found on host, skipping"; \
		done; \
		/userdata/system/apps/rex/debian-enter.sh --root "ldconfig"'
	@echo "Enabling bookworm-backports (for tmux 3.5a — extended-keys-format)..."
	ssh brick-root '/userdata/system/apps/rex/debian-enter.sh --root "echo deb http://deb.debian.org/debian bookworm-backports main > /etc/apt/sources.list.d/backports.list"'
	@echo "Configuring NodeSource apt repo for Node 20 (Debian bookworm ships Node 18 which lacks ES2024 regex-v; pi-coding-agent crashes on import. See notes/research/brick/debian-chroot-on-knulli.md)..."
	ssh brick-root '/userdata/system/apps/rex/debian-enter.sh --root "\
		apt update -qq && \
		apt install -y -qq gnupg ca-certificates curl && \
		[ -f /usr/share/keyrings/nodesource.gpg ] || { \
			curl -fsSL -o /tmp/nodesource.key https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key && \
			gpg --dearmor --yes -o /usr/share/keyrings/nodesource.gpg /tmp/nodesource.key && \
			rm -f /tmp/nodesource.key; \
		} && \
		[ -f /etc/apt/sources.list.d/nodesource.list ] || \
			echo \"deb [signed-by=/usr/share/keyrings/nodesource.gpg] https://deb.nodesource.com/node_20.x nodistro main\" > /etc/apt/sources.list.d/nodesource.list"'
	@echo "Installing apt packages + claude-code inside chroot..."
	ssh brick-root '/userdata/system/apps/rex/debian-enter.sh --root "apt update -qq && apt install -y -qq sudo libsdl2-2.0-0 openssh-server locales alsa-utils nodejs && apt install -y -qq -t bookworm-backports tmux && npm install -g @anthropic-ai/claude-code 2>&1 | tail -5 && which claude && claude --version && node --version && npm --version && tmux -V"'
	@echo "Generating en_US.UTF-8 locale in chroot..."
	ssh brick-root '/userdata/system/apps/rex/debian-enter.sh --root "\
		grep -q ^en_US.UTF-8 /etc/locale.gen || echo en_US.UTF-8 UTF-8 >> /etc/locale.gen && \
		locale-gen en_US.UTF-8 >/dev/null && \
		update-locale LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8"'
	@echo "Creating rex user (sudo NOPASSWD, groups: sudo,audio,video)..."
	ssh brick-root '/userdata/system/apps/rex/debian-enter.sh --root "\
		id rex >/dev/null 2>&1 || useradd -m -s /bin/bash -G sudo,audio,video rex && \
		usermod -aG sudo,audio,video rex && \
		getent group inet >/dev/null 2>&1 || groupadd -g 3003 inet && \
		usermod -aG inet rex && \
		echo \"rex ALL=(ALL) NOPASSWD:ALL\" > /etc/sudoers.d/rex-nopasswd && \
		chmod 440 /etc/sudoers.d/rex-nopasswd && \
		for item in .cache .claude .claude.json .local .npm .tmux.conf bin notes projects; do \
			[ -e /root/\$$item ] && [ ! -e /home/rex/\$$item ] && cp -a /root/\$$item /home/rex/ || true; \
		done && \
		{ [ -d /root/.config ] && mkdir -p /home/rex/.config && cp -rn /root/.config/. /home/rex/.config/ 2>/dev/null; true; } && \
		chown -R rex:rex /home/rex"'
	@echo "Configuring sshd in chroot (port 2222, pubkey auth, rex user)..."
	ssh brick-root '/userdata/system/apps/rex/debian-enter.sh --root "\
		mkdir -p /etc/ssh/sshd_config.d && \
		echo Port 2222 > /etc/ssh/sshd_config.d/rex.conf && \
		echo PermitRootLogin prohibit-password >> /etc/ssh/sshd_config.d/rex.conf && \
		mkdir -p /root/.ssh && chmod 700 /root/.ssh && \
		cp /userdata/system/.ssh/authorized_keys /root/.ssh/authorized_keys && \
		chmod 600 /root/.ssh/authorized_keys && \
		mkdir -p /home/rex/.ssh && chmod 700 /home/rex/.ssh && \
		cp /userdata/system/.ssh/authorized_keys /home/rex/.ssh/authorized_keys && \
		chmod 600 /home/rex/.ssh/authorized_keys && \
		chown -R rex:rex /home/rex/.ssh"'
	@echo "Done. To start sshd on boot, add this line to /userdata/system/custom.sh:"
	@echo "  /userdata/system/apps/rex/debian-sshd.sh"
	@echo "Connect with: ssh brick"

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
