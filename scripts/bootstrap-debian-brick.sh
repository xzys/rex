#!/usr/bin/env bash
# Bootstrap a pre-baked Debian arm64 rootfs for TrimUI Brick chroot.
# Runs debootstrap + full package install inside a linux/arm64 Docker container,
# then tars up the rootfs. The resulting chroot is ready-to-run — no apt or npm
# needed on first boot.
#
# Does NOT include claude-code (@anthropic-ai/claude-code is proprietary and
# cannot be redistributed). Users install it on-device via the More menu.
#
# Host can be macOS; on Apple Silicon this runs natively without QEMU emulation.

set -euo pipefail

SUITE="${SUITE:-bookworm}"
OUT="${OUT:-bin/brick-debian.tar.gz}"
MIRROR="${MIRROR:-http://deb.debian.org/debian}"

# Base packages for debootstrap --include. tmux is included from base (3.3a)
# and upgraded to 3.5a from backports in the post-debootstrap step.
INCLUDES="ca-certificates,curl,wget,git,vim,tmux,htop,openssh-client,gnupg"
INCLUDES="$INCLUDES,build-essential,python3,python3-pip,pkg-config,less,file,procps"
INCLUDES="$INCLUDES,libsdl2-2.0-0,sudo,openssh-server,locales,alsa-utils"

mkdir -p "$(dirname "$OUT")"
OUTDIR="$(cd "$(dirname "$OUT")" && pwd)"
OUTNAME="$(basename "$OUT")"

docker run -i --rm --platform linux/arm64 \
    -v "$OUTDIR:/output" \
    debian:"$SUITE" bash -eux <<DOCKER_EOF
# Strict umask causes permission problems later (rex can't read root-installed
# files). Set a relaxed umask before any package installation.
umask 022

apt-get update -qq
apt-get install -y -qq --no-install-recommends debootstrap gnupg

# --- Stage 1: debootstrap minimal base --------------------------------
debootstrap \
    --arch=arm64 \
    --variant=minbase \
    --include='$INCLUDES' \
    '$SUITE' /rootfs '$MIRROR'

# --- Stage 2: configure base system inside chroot ---------------------
echo 'nameserver 8.8.8.8' > /rootfs/etc/resolv.conf
echo 'brick-debian' > /rootfs/etc/hostname

cat > /rootfs/etc/apt/sources.list <<EOF
deb $MIRROR $SUITE main contrib non-free non-free-firmware
deb $MIRROR $SUITE-updates main contrib non-free non-free-firmware
deb http://security.debian.org/debian-security $SUITE-security main contrib non-free non-free-firmware
EOF

# apt drops to the _apt user for http methods, which fails DNS inside a
# Knulli host chroot. Run acquire methods as root instead.
echo 'APT::Sandbox::User "root";' > /rootfs/etc/apt/apt.conf.d/00no-sandbox

# --- Stage 3: bookworm-backports (tmux 3.5a) --------------------------
echo 'deb http://deb.debian.org/debian bookworm-backports main' \
    > /rootfs/etc/apt/sources.list.d/backports.list

# --- Stage 4: NodeSource for Node 20 ----------------------------------
# Debian bookworm ships Node 18 which lacks the ES2024 regex-v flag;
# pi-coding-agent crashes on import. Install Node 20 from NodeSource.
chroot /rootfs bash -eux <<'CHROOT_NODE'
curl -fsSL -o /tmp/nodesource.key \
    https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key
gpg --dearmor --yes -o /usr/share/keyrings/nodesource.gpg /tmp/nodesource.key
rm -f /tmp/nodesource.key
echo 'deb [signed-by=/usr/share/keyrings/nodesource.gpg] https://deb.nodesource.com/node_20.x nodistro main' \
    > /etc/apt/sources.list.d/nodesource.list
CHROOT_NODE

# --- Stage 5: install all packages ------------------------------------
chroot /rootfs bash -eux <<'CHROOT_INSTALL'
# Update with all repos (base + backports + nodesource)
apt-get update -qq

# Install tmux 3.5a from backports (upgrades base 3.3a)
apt-get install -y -qq -t bookworm-backports tmux

# Install Node 20 (replaces Debian's Node 18)
apt-get install -y -qq nodejs

# npm global install (pi coding agent)
npm install -g @earendil-works/pi-coding-agent 2>&1 | tail -5

# Verify key system packages
node --version
npm --version
tmux -V
CHROOT_INSTALL

# --- Stage 6: locale --------------------------------------------------
chroot /rootfs bash -eux <<'CHROOT_LOCALE'
echo 'en_US.UTF-8 UTF-8' >> /etc/locale.gen
locale-gen en_US.UTF-8
update-locale LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8
CHROOT_LOCALE

# --- Stage 7: rex user + groups ---------------------------------------
chroot /rootfs bash -eux <<'CHROOT_USER'
# Create rex user with passwordless sudo
useradd -m -s /bin/bash -G sudo,audio,video rex
echo 'rex ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/rex-nopasswd
chmod 440 /etc/sudoers.d/rex-nopasswd

# inet group (GID 3003) — required for non-root network sockets on the
# Brick's Allwinner A133+ BSP kernel (CONFIG_ANDROID_PARANOID_NETWORK=y).
groupadd -g 3003 inet
usermod -aG inet rex
CHROOT_USER

# --- Stage 8: sshd config (port 2222, pubkey auth) --------------------
chroot /rootfs bash -eux <<'CHROOT_SSHD'
mkdir -p /etc/ssh/sshd_config.d
cat > /etc/ssh/sshd_config.d/rex.conf <<EOF
Port 2222
PermitRootLogin prohibit-password
EOF

# Create .ssh dirs (authorized_keys will be copied from Knulli host
# at install time if present)
mkdir -p /home/rex/.ssh /root/.ssh
chmod 700 /home/rex/.ssh /root/.ssh
chown rex:rex /home/rex/.ssh
CHROOT_SSHD

# --- Stage 9: fix npm permissions -------------------------------------
# npm install -g as root creates files/dirs with root-only perms.
# Make them world-readable so the rex user can run pi.
# NodeSource places global modules under /usr/lib/node_modules.
chroot /rootfs bash -eux <<'CHROOT_PERM'
chmod 755 /usr/lib/node_modules
chmod -R go+rX /usr/lib/node_modules/@earendil-works 2>/dev/null || true
CHROOT_PERM

# --- Stage 10: cleanup ------------------------------------------------
chroot /rootfs apt-get clean
rm -rf /rootfs/var/lib/apt/lists/*
rm -rf /rootfs/root/.npm /rootfs/root/.cache

# --- Stage 11: package into tarball -----------------------------------
tar czf /output/$OUTNAME -C /rootfs .
DOCKER_EOF

echo
echo "Rootfs tarball: $OUT"
ls -lh "$OUT"
