# Installation: Boyhom R36S (legacy)

> **Note:** The R36S is not actively supported. The TrimUI Brick is the primary target — see [installation.md](installation.md). This document is kept as a reference for the existing R36S install path.

Rex runs natively on Boyhom R36S devices running dArkOSRE, under Debian trixie and systemd. dArkOSRE uses Debian trixie, so Rex installs with `apt` directly on the host system.

## Package install

Download `rex-r36s_<version>_arm64.deb` from the [Rex releases page](https://github.com/xzys/rex/releases) on the R36S, then install and enable the service:

```bash
# Run on the R36S as user "ark".
curl -fLO https://github.com/xzys/rex/releases/download/v<version>/rex-r36s_<version>_arm64.deb
sudo apt install ./rex-r36s_<version>_arm64.deb
sudo systemctl enable --now rex.service
```

The package installs:

- `/usr/bin/rex`, the Rex binary
- `/lib/systemd/system/rex.service`, the systemd service unit
- `/opt/system/Switch to Rex.sh` and `/opt/system/Switch to Rex Permanently.sh`, the boot-mode switchers
- Templates under `/usr/share/rex/`

The service is not enabled automatically during package installation. You opt in by running `sudo systemctl enable --now rex.service`.

`apt` resolves runtime dependencies from the standard dArkOSRE package sources. Required dependencies include tmux 3.5 or newer, sudo, and locales. Recommended dependencies include Node.js 20 or newer, alsa-utils, and openssh-server. No backports or third-party repositories are required on dArkOSRE.

To upgrade later, download the newer `.deb` and run the same `sudo apt install ./rex-r36s_<version>_arm64.deb` command.

## Verify the install

```bash
rex --version          # prints the build SHA
which rex              # should print /usr/bin/rex
systemctl status rex   # the service should be active
```

## Troubleshooting

### `rex.service` starts and exits immediately

This is usually a TTY acquisition problem. Check the service logs first:

```bash
journalctl -u rex.service -b
```

If the logs point to terminal setup, compare the installed unit with the current service configuration in the Rex repository notes.
