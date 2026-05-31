# Installation

Rex ships as four artifacts for two supported devices. Pick the path that
matches what you already have.

## Which install do I need?

```
TrimUI Brick
├── Empty / new SD card           →  Overlay image (easiest)
├── Existing Knulli, no rex       →  Tarball over ssh
└── Existing rex (upgrade)        →  .deb inside chroot

Boyhom R36S
└── dArkOSRE (any state)          →  .deb on host
```

## Supported devices

| Device | Firmware | Notes |
|---|---|---|
| TrimUI Brick | Knulli | Rex runs inside a Debian arm64 chroot at `/userdata/debian`. |
| Boyhom R36S | dArkOSRE | Rex runs natively. Debian trixie + systemd. |

Other devices are not officially supported — primarily because mic input
(used by /voice) isn't portable across hardware yet.

## Path 1: Brick — overlay image (first-time, easiest)

Download `knulli-rex-brick-<version>.img.gz` from the latest GitHub
release, then flash it to a microSD card.

```bash
# macOS / Linux:
gunzip -c knulli-rex-brick-<version>.img.gz | sudo dd of=/dev/diskN bs=4M status=progress

# Or use balenaEtcher with the .img.gz directly — point it at the file
# and let it pick the SD card.
```

Insert the SD card into the Brick. First boot:

1. Knulli's `userdata` partition resizes to fill the card (~30s).
2. The rex payload extracts from `/userdata/rex-bundle/`.
3. EmulationStation comes up. Open the wifi config from the ES menu and
   join your network.
4. Once wifi is connected, the installer continues automatically:
   extracts the bundled Debian chroot (~2 min), runs `apt` + `npm` to set
   up tmux, Node 20, sshd, claude-code, etc. (~5–10 min on first run).
5. Rex autostarts. Subsequent boots go straight to rex (~2s).

The chroot tarball is bundled in the image — no separate download
required during install.

If wifi isn't configured in time, the chroot setup step (apt + npm)
fails and retries on the next boot. ES stays up the whole time so you
can fix wifi without re-flashing.

Logs: `/var/log/rex-firstboot.log` on the device.

## Path 2: Brick — tarball over ssh (existing Knulli, no rex)

Use this if you've already set up Knulli, are connected to wifi, and want
to install rex without re-flashing.

Download `rex-brick-<version>.tar.gz` from the latest GitHub release,
then on your laptop:

```bash
# Streams the tarball to the Brick and runs the installer remotely:
./install.sh --remote brick-root
```

You need ssh key auth to `brick-root` (root@<brick-ip>:22) set up first.

Or do it manually:

```bash
scp rex-brick-<version>.tar.gz brick-root:/tmp/
ssh brick-root '
  cd /tmp &&
  tar xzf rex-brick-<version>.tar.gz &&
  cd rex-brick-<version> &&
  REX_CHROOT_URL=https://github.com/xzys/rex/releases/download/v<version>/brick-debian.tar.gz \
    sudo ./install.sh
'
```

`REX_CHROOT_URL` must point at the Debian arm64 rootfs tarball published
alongside the release. The installer downloads it (~70MB) and extracts
to `/userdata/debian/` before running the apt setup.

If `/userdata/debian/` already has a valid chroot (`/etc/debian_version`
present), the installer skips the chroot download and re-runs only the
binary install + idempotent chroot setup steps.

## Path 3: Brick — .deb inside chroot (upgrades)

Use this to upgrade rex on a Brick that already has it installed.

Inside the chroot (ssh in as `rex` on port 2222, or use `brick`):

```bash
ssh brick
curl -fLO https://github.com/xzys/rex/releases/download/v<version>/rex-brick_<version>_arm64.deb
sudo apt install ./rex-brick_<version>_arm64.deb
```

This replaces `/usr/bin/rex` and refreshes templates under
`/usr/share/rex/`. Restart the rex session to pick up the new binary
(`/userdata/system/apps/rex/stop-rex.sh && ./start-rex.sh`, or reboot).

The .deb is a binary upgrade only — it does **not** touch Knulli-host
files (`/userdata/system/apps/rex/`, `Rex.sh`, `custom.sh`). For full
upgrades of host-side files, re-run path 2 with a current tarball.

## Path 4: R36S — .deb on dArkOSRE

dArkOSRE is Debian trixie, so apt is native. Download
`rex-r36s_<version>_arm64.deb` from the GitHub release.

```bash
# On the R36S (as user 'ark'):
curl -fLO https://github.com/xzys/rex/releases/download/v<version>/rex-r36s_<version>_arm64.deb
sudo apt install ./rex-r36s_<version>_arm64.deb
sudo systemctl enable --now rex.service
```

This installs:

- `/usr/bin/rex` — the binary
- `/lib/systemd/system/rex.service` — the systemd unit
- `/opt/system/Switch to Rex.sh` + `Switch to Rex Permanently.sh` — boot mode toggles
- Templates at `/usr/share/rex/`

The service is **not** enabled automatically — you opt in with the
`systemctl enable` step above.

Apt resolves the runtime deps (tmux ≥ 3.5, sudo, locales, recommended:
nodejs ≥ 20, alsa-utils, openssh-server) from dArkOSRE's standard
sources. No backports or third-party repos needed.

Subsequent upgrades: same command with a newer .deb.

## Verifying the install

After any install, on-device:

```bash
rex --version          # prints the build SHA
which rex              # should be /usr/bin/rex
systemctl status rex   # (R36S only)
```

On the Brick, the chroot enter helper is at
`/userdata/system/apps/rex/debian-enter.sh` if you need to drop into the
chroot from the Knulli host side.

## Troubleshooting

**Brick: rex doesn't autostart after first flash.**
Check `/var/log/rex-firstboot.log` over ssh as `root`. The most common
cause is wifi not configured yet — fix wifi, reboot, the installer
retries.

**Brick: chroot setup step fails repeatedly.**
Apt or npm couldn't reach the network. Verify `ip route show default`
inside `debian-enter.sh --root` returns a default gateway. DNS issues
sometimes need a resolv.conf nudge.

**.deb refuses to install (`dependency not satisfiable: tmux (>= 3.5)`).**
On the Brick chroot, `tmux 3.5a` requires bookworm-backports. If
`install-brick.sh` was skipped, you'll need to add the backports source
manually: `echo 'deb http://deb.debian.org/debian bookworm-backports main' > /etc/apt/sources.list.d/backports.list && apt update`.

**R36S: rex.service starts but exits immediately.**
Likely a tty acquisition issue (the controlling terminal). See
`darkos-re-setup.md` in the internal notes for the systemd unit
configuration history.

## See also

- [docs/architecture.md](architecture.md) — how rex is structured
- Release artifacts: <https://github.com/xzys/rex/releases>
