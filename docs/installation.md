# Installation

Rex is distributed as release artifacts for two supported handhelds:

- TrimUI Brick running Knulli
- Boyhom R36S running dArkOSRE

Choose the install path that matches your device and current setup.

## Choose an install path

| Device and current state | Artifact | Install path |
| --- | --- | --- |
| TrimUI Brick with a new or empty SD card | `knulli-rex-brick-<version>.img.gz` | Flash the overlay image. This is the easiest first-time install. |
| TrimUI Brick with Knulli already installed | `rex-brick-<version>.tar.gz` | Copy the tarball over SSH and run the installer. |
| TrimUI Brick with Rex already installed | `rex-brick_<version>_arm64.deb` | Upgrade Rex inside the Debian chroot. |
| Boyhom R36S running dArkOSRE | `rex-r36s_<version>_arm64.deb` | Install the Debian package directly on the device. |

Download release artifacts from the [Rex releases page](https://github.com/xzys/rex/releases). Replace `<version>` in the commands below with the release version you downloaded.

## Supported devices

| Device | Firmware | Runtime environment |
| --- | --- | --- |
| TrimUI Brick | Knulli | Rex runs inside a Debian arm64 chroot at `/userdata/debian`. |
| Boyhom R36S | dArkOSRE | Rex runs natively under Debian trixie and systemd. |

Other devices are not officially supported. The main blocker is portable microphone input, which Rex uses for `/voice`.

## Path 1: TrimUI Brick image install

Use this path for a first-time Brick install, especially if you are starting with a new or empty SD card.

### Flash the image

Download `knulli-rex-brick-<version>.img.gz`, then flash it to a microSD card.

```bash
# macOS / Linux:
gunzip -c knulli-rex-brick-<version>.img.gz | sudo dd of=/dev/diskN bs=4M status=progress
```

Replace `/dev/diskN` with the SD card device. Double-check the target before running `dd`; writing to the wrong disk can destroy data.

You can also flash the `.img.gz` file directly with balenaEtcher.

### First boot

Insert the SD card into the Brick and boot it. The first boot runs in stages:

1. Knulli expands and initializes the `userdata` partition to fill the SD card. This takes about 30 seconds.
2. The Rex payload and bundled Debian chroot are copied from the boot partition into `userdata`.
3. The installer extracts the preconfigured Debian chroot and installs the Knulli-side Rex launcher files. Network access is not required for this first-boot install.
4. Rex starts automatically. Configure Wi-Fi from Rex's built-in Wi-Fi settings.
5. Future boots skip the setup work and start Rex directly.

You may briefly see EmulationStation during first boot while the startup hook hands off to Rex. Rex is also installed as an EmulationStation fallback launcher under Tools → Rex, so you can return to ES for recovery and launch Rex again from there if needed.

The Debian chroot tarball and dependencies are included in the image, so this path does not require a separate chroot download or first-boot package install.

First-boot logs are written to `/var/log/rex-firstboot.log` on the device.

## Path 2: TrimUI Brick tarball install over SSH

Use this path if Knulli is already installed on the Brick and you want to add Rex without reflashing.

Before you start, make sure:

- The Brick is connected to Wi-Fi.
- You can SSH to the Knulli host as `root`.
- Your laptop has an SSH alias such as `brick-root` for `root@<brick-ip>` on port 22.

Download `rex-brick-<version>.tar.gz`, extract it on your laptop, and run the remote installer:

```bash
./install.sh --remote brick-root
```

The remote installer streams the release payload to the Brick and runs the install script there.

### Manual install

If you prefer to run the steps yourself, copy the tarball to the Brick and execute the installer over SSH:

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

`REX_CHROOT_URL` must point to the Debian arm64 root filesystem tarball published with the same release. The installer downloads it, extracts it to `/userdata/debian/`, and then runs the chroot setup.

If `/userdata/debian/` already contains a valid Debian chroot, identified by `/etc/debian_version`, the installer skips the chroot download. It still refreshes the Rex binary and reruns the idempotent chroot setup steps.

## Path 3: TrimUI Brick package upgrade

Use this path to upgrade Rex on a Brick that already has Rex installed.

Run the package install inside the Debian chroot. The examples below assume an SSH alias named `brick` that connects to the chroot as the `rex` user on port 2222.

```bash
ssh brick
curl -fLO https://github.com/xzys/rex/releases/download/v<version>/rex-brick_<version>_arm64.deb
sudo apt install ./rex-brick_<version>_arm64.deb
```

This package replaces `/usr/bin/rex` and refreshes templates under `/usr/share/rex/`.

Restart Rex after the package install so the running session uses the new binary:

```bash
/userdata/system/apps/rex/stop-rex.sh && /userdata/system/apps/rex/start-rex.sh
```

You can also reboot the device.

The Brick `.deb` is a chroot-only binary upgrade. It does **not** update Knulli host files such as `/userdata/system/apps/rex/`, `Rex.sh`, or `custom.sh`. To refresh those host-side files, rerun the tarball installer from Path 2 with the current release.

## Path 4: R36S package install

Use this path for Boyhom R36S devices running dArkOSRE. dArkOSRE uses Debian trixie, so Rex installs with `apt` directly on the host system.

Download `rex-r36s_<version>_arm64.deb` on the R36S, then install and enable the service:

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

After any install or upgrade, check the installed binary on the device:

```bash
rex --version          # prints the build SHA
which rex              # should print /usr/bin/rex
```

On R36S, also check the systemd service:

```bash
systemctl status rex
```

On Brick, use `/userdata/system/apps/rex/debian-enter.sh` if you need to enter the Debian chroot from the Knulli host.

## Troubleshooting

### Brick: Rex does not start after the first image boot

Check `/var/log/rex-firstboot.log` over SSH as `root`. The image install should not need Wi-Fi during first boot; failures usually mean the bundled payload did not extract, the chroot did not unpack, or the launcher files were not installed.

If EmulationStation is visible, check whether the fallback launcher exists at `/userdata/roms/tools/Rex.sh` and try launching Rex from Tools → Rex.

### Brick: Wi-Fi does not work inside Rex

Rex manages Wi-Fi through ConnMan on the Knulli host. If networks do not appear or password entry fails, reboot once and try again. If the problem persists, check the Rex logs and confirm the ConnMan D-Bus policy was installed:

```bash
ls -l /etc/dbus-1/system.d/connman-rex.conf
```

### Package install fails with `dependency not satisfiable: tmux (>= 3.5)`

On the Brick chroot, tmux 3.5a comes from Debian bookworm-backports. The Brick installer normally configures this repository. If you skipped that setup, add the backports source manually:

```bash
echo 'deb http://deb.debian.org/debian bookworm-backports main' | sudo tee /etc/apt/sources.list.d/backports.list
sudo apt update
```

Then rerun the package install.

### R36S: `rex.service` starts and exits immediately

This is usually a TTY acquisition problem. Check the service logs first:

```bash
journalctl -u rex.service -b
```

If the logs point to terminal setup, compare the installed unit with the current service configuration in the Rex repository notes.

## See also

- [Rex architecture](https://github.com/xzys/rex/blob/master/ARCHITECTURE.md)
- [Rex releases](https://github.com/xzys/rex/releases)
