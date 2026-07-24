.PHONY: build-local debug install-system-files install-dotfiles install-bin install-systemd install-stt-sherpa install-stt-parakeet install-stt-models install-kitten-tts-brick sync-r36s sync-auth-r36s sync-brick bootstrap-debian sync-debian setup-debian setup-debian-extras setup-debian-zot package package-brick package-deb package-deb-brick package-deb-r36s package-overlay build-docker-termsp screenshot-brick textdump-brick

build-local: ## Build for local machine with keyboard input
	cd src && go build -tags keyboard -o ../rex-local

debug:
	cd src && dlv debug . -- --server

build-docker-r36s: ## Build for R36S using Docker
	# On Docker Desktop, plain `docker build` may leave only a manifest-list tag
	# that `docker run` then fails to resolve locally. `buildx --load` forces a
	# runnable linux/amd64 image into the local image store.
	docker buildx build --platform linux/amd64 --load -f src/Dockerfile -t r36s-keyboard-builder src/
	docker run --rm --platform linux/amd64 \
		-v go-mod-cache:/root/go/pkg/mod \
		-v go-build-cache:/root/.cache/go-build \
		-v $$(pwd)/src:/app \
		-v $$(pwd)/bin:/output \
		-w /app \
		r36s-keyboard-builder \
		go build -tags sdl -o /output/rex

sync-r36s:
	@echo "Syncing rex binary and scripts to r36s..."
	ssh r36s 'mkdir -p ~/bin ~/.config/rex'
	rsync bin/rex r36s:bin/
	rsync config.yaml r36s:~/.config/rex/

sync-auth-r36s: ## Sync Codex, pi, and Claude auth to r36s
	@echo "Syncing Codex, pi, and Claude auth to r36s..."
	@test -f "$(HOME)/.codex/auth.json" || { echo "Missing $(HOME)/.codex/auth.json"; exit 1; }
	@test -f "$(HOME)/.pi/agent/auth.json" || { echo "Missing $(HOME)/.pi/agent/auth.json"; exit 1; }
	@command -v security >/dev/null || { echo "macOS 'security' CLI not found"; exit 1; }
	ssh r36s 'umask 077 && mkdir -p ~/.codex ~/.pi/agent ~/.claude'
	scp "$(HOME)/.codex/auth.json" r36s:/home/ark/.codex/auth.json
	scp "$(HOME)/.pi/agent/auth.json" r36s:/home/ark/.pi/agent/auth.json
	security find-generic-password -s "Claude Code-credentials" -w | ssh r36s 'umask 077 && cat > /home/ark/.claude/.credentials.json'
	ssh r36s 'chmod 600 ~/.codex/auth.json ~/.pi/agent/auth.json ~/.claude/.credentials.json'

build-docker-brick: ## Build rex for TrimUI Brick (cross-compile ARM64, SDL+brick tags)
	docker buildx build --platform linux/amd64 --load -f src/Dockerfile -t brick-rex-builder src/
	docker run --rm --platform linux/amd64 \
		-v go-mod-cache:/root/go/pkg/mod \
		-v go-build-cache:/root/.cache/go-build \
		-v $$(pwd)/src:/app \
		-v $$(pwd)/bin:/output \
		-w /app \
		brick-rex-builder \
		go build -tags "sdl brick sherpa" -o /output/rex-brick

build-docker-termsp: ## Build TermSP for TrimUI Brick (cross-compile ARM64, outputs bin/termsp)
	docker buildx build --platform linux/amd64 --load -t termsp-builder ext/TermSP/
	docker run --rm --platform linux/amd64 \
		-v $$(pwd)/ext/TermSP:/app \
		-v $$(pwd)/bin:/output \
		-w /app \
		termsp-builder \
		bash build.sh

screenshot-brick: ## Trigger TermSP screenshot on Brick and pull bin/termsp-screenshot.png
	ssh brick-root 'pkill -USR1 termsp && sleep 1 && ls -la /tmp/termsp-screenshot.png'
	scp brick-root:/tmp/termsp-screenshot.png bin/termsp-screenshot.png
	@echo "Saved bin/termsp-screenshot.png"
	@command -v open >/dev/null && open bin/termsp-screenshot.png || true

textdump-brick: ## Trigger TermSP screen-text dump on Brick and pull bin/termsp-text.txt
	ssh brick-root 'pkill -USR2 termsp && sleep 1 && ls -la /tmp/termsp-text.txt'
	scp brick-root:/tmp/termsp-text.txt bin/termsp-text.txt
	@echo "Saved bin/termsp-text.txt"

sync-brick: ## Deploy rex (into Debian chroot), termsp (Knulli), fonts, and launch scripts to TrimUI Brick
	@echo "Syncing to TrimUI Brick..."
	ssh brick-root 'mkdir -p /userdata/system/apps/rex/fonts /userdata/system/apps/rex/lib /userdata/roms/ports /userdata/debian/home/rex/bin /userdata/debian/home/rex/.config/rex'
	scp bin/rex-brick brick-root:/userdata/debian/home/rex/bin/rex
	scp bin/termsp brick-root:/userdata/system/apps/rex/termsp
	scp ext/TermSP/TermSP/libs/libsdlfox.so ext/TermSP/TermSP/libs/libvterm.so brick-root:/userdata/system/apps/rex/lib/
	ssh brick-root 'ln -sf libvterm.so /userdata/system/apps/rex/lib/libvterm.so.0'
	scp ext/fonts/*.ttf brick-root:/userdata/system/apps/rex/fonts/
	scp ext/brick/Rex.sh brick-root:/userdata/roms/tools/Rex.sh
	scp ext/brick/start-rex.sh ext/brick/stop-rex.sh ext/brick/debian-rex.sh ext/brick/debian-sshd.sh ext/brick/connman-rex.conf brick-root:/userdata/system/apps/rex/
	scp config.yaml.brick.example brick-root:/userdata/system/apps/rex/config.yaml.brick.example
	scp config/dot/tmux.conf brick-root:/userdata/debian/home/rex/.tmux.conf
	scp config/dot/tmux-wifi.sh brick-root:/userdata/debian/home/rex/.tmux-wifi.sh
	scp ext/kitten-tts/kitten-speak brick-root:/userdata/debian/home/rex/bin/kitten-speak
	ssh brick-root '[ -f /userdata/debian/home/rex/.config/rex/config.yaml ] || cp /userdata/system/apps/rex/config.yaml.brick.example /userdata/debian/home/rex/.config/rex/config.yaml'
	ssh brick-root 'chmod +x /userdata/debian/home/rex/bin/rex /userdata/debian/home/rex/bin/kitten-speak /userdata/debian/home/rex/.tmux-wifi.sh /userdata/system/apps/rex/termsp /userdata/roms/tools/Rex.sh /userdata/system/apps/rex/start-rex.sh /userdata/system/apps/rex/stop-rex.sh /userdata/system/apps/rex/debian-rex.sh /userdata/system/apps/rex/debian-sshd.sh'
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

install-kitten-tts-brick: ## Install optional KittenTTS binary + model into the Brick Debian chroot (ssh alias: brick)
	@echo "Installing KittenTTS into Brick Debian chroot via ssh alias 'brick'..."
	tar czf - -C ext/kitten-tts install-brick.sh kitten-speak | ssh brick 'set -e; d=$$(mktemp -d); tar xzf - -C "$$d"; bash "$$d/install-brick.sh"; rm -rf "$$d"'

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

setup-debian: ## (Re)install Debian packages, sherpa libs, rex user, and pi inside the chroot (idempotent)
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
	@echo "Installing apt packages + pi inside chroot..."
	ssh brick-root '/userdata/system/apps/rex/debian-enter.sh --root "apt update -qq && apt install -y -qq sudo libsdl2-2.0-0 openssh-server locales alsa-utils nodejs && apt install -y -qq -t bookworm-backports tmux && npm install -g @earendil-works/pi-coding-agent 2>&1 | tail -5 && which pi && pi --version && node --version && npm --version && tmux -V"'
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

# Optional curated tools for the chroot. Idempotent — safe to re-run.
# apt: ncmpcpp+mpd (music), fzf (fuzzy picker), htop (process viewer)
# .deb downloads: aptui (apt TUI), bottom/btm (system monitor)
APTUI_VERSION := 0.7.0
BOTTOM_VERSION := 0.12.3
ZOT_VERSION ?= 0.2.93
setup-debian-extras: ## Install curated extras into the chroot (ncmpcpp+mpd, btm, aptui, fzf, htop)
	@echo "Installing apt extras (ncmpcpp mpd fzf htop)..."
	ssh brick-root '/userdata/system/apps/rex/debian-enter.sh --root "apt update -qq && apt install -y -qq ncmpcpp mpd fzf htop"'
	@echo "Installing aptui $(APTUI_VERSION) (.deb)..."
	ssh brick 'set -e; cd /tmp && \
		[ -f aptui_$(APTUI_VERSION)_linux_arm64.deb ] || curl -fsSLO https://github.com/mexirica/aptui/releases/download/v$(APTUI_VERSION)/aptui_$(APTUI_VERSION)_linux_arm64.deb && \
		sudo dpkg -i aptui_$(APTUI_VERSION)_linux_arm64.deb || sudo apt-get install -f -y'
	@echo "Installing bottom $(BOTTOM_VERSION) (.deb, provides btm)..."
	ssh brick 'set -e; cd /tmp && \
		[ -f bottom_$(BOTTOM_VERSION)-1_arm64.deb ] || curl -fsSLO https://github.com/ClementTsang/bottom/releases/download/$(BOTTOM_VERSION)/bottom_$(BOTTOM_VERSION)-1_arm64.deb && \
		sudo dpkg -i bottom_$(BOTTOM_VERSION)-1_arm64.deb || sudo apt-get install -f -y'
	@echo "Done. Run on brick: aptui, btm, ncmpcpp"

setup-debian-zot: build-docker-ext-zot ## Install zot (coding agent) into the chroot
	@echo "Installing zot $(ZOT_VERSION) into chroot..."
	scp bin/zot brick-root:/userdata/debian/home/rex/bin/zot
	ssh brick-root 'chmod +x /userdata/debian/home/rex/bin/zot'
	@echo "zot $(ZOT_VERSION) installed. Configure API keys via env vars or ~/.local/state/zot/"
	@echo "Run on brick: zot"

build-docker-ext-%: ## Build program from ext/
	docker buildx build --platform linux/amd64 --load -f ext/$*/Dockerfile -t r36s-$(shell echo $* | tr '[:upper:]' '[:lower:]')-builder ext/$*
	docker run --rm --platform linux/amd64 \
		-v go-mod-cache:/root/go/pkg/mod \
		-v go-build-cache:/root/.cache/go-build \
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

package: package-brick ## Build install tarball for Brick (used by the overlay image)

package-brick: ## Build dist/rex-brick-<v>.tar.gz (requires bin/rex-brick + bin/termsp)
	bash scripts/package.sh brick

package-overlay: package-brick bin/brick-debian.tar.gz ## Build dist/rex-brick-overlay-<v>.tar.gz (scripts + payload + chroot)
	bash scripts/knulli/package-overlay.sh

bin/brick-debian.tar.gz:
	@echo "bin/brick-debian.tar.gz missing — running 'make bootstrap-debian'..."
	$(MAKE) bootstrap-debian

package-deb-brick: ## Build dist/rex-brick_<v>_arm64.deb (needs nfpm: brew install nfpm)
	@command -v nfpm >/dev/null 2>&1 || { echo "nfpm not found — install with: brew install nfpm"; exit 1; }
	@test -f bin/rex-brick || { echo "bin/rex-brick missing — run 'make build-docker-brick' first"; exit 1; }
	@mkdir -p dist
	REX_VERSION=$$(git rev-parse --short HEAD 2>/dev/null || echo dev) \
		nfpm pkg --config packaging/nfpm.brick.yaml --packager deb --target dist/

package-deb-r36s: ## Build dist/rex-r36s_<v>_arm64.deb (needs nfpm: brew install nfpm)
	@command -v nfpm >/dev/null 2>&1 || { echo "nfpm not found — install with: brew install nfpm"; exit 1; }
	@test -f bin/rex || { echo "bin/rex missing — run 'make build-docker-r36s' first"; exit 1; }
	@mkdir -p dist
	REX_VERSION=$$(git rev-parse --short HEAD 2>/dev/null || echo dev) \
		nfpm pkg --config packaging/nfpm.r36s.yaml --packager deb --target dist/

package-deb: package-deb-brick package-deb-r36s ## Build both .deb artifacts
