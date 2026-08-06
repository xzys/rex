# Installation

Rex runs on the TrimUI Brick with Knulli firmware, inside a Debian arm64 chroot at `/userdata/debian`.

## Choose an install path

| Current state | Artifact | Install path |
| --- | --- | --- |
| New or empty SD card | `knulli-rex-brick-<version>.img.gz` | Flash the overlay image. This is the easiest first-time install. |
| Knulli already installed | `rex-brick-<version>.tar.gz` | Copy the tarball over SSH and run the installer. |
| Rex already installed | `rex-brick_<version>_arm64.deb` | Upgrade Rex inside the Debian chroot. |

Download release artifacts from the [Rex releases page](https://github.com/xzys/rex/releases). Replace `<version>` in the commands below with the release version you downloaded.

## Path 1: Image install

Use this path for a first-time install, especially if you are starting with a new or empty SD card.

### Flash the image

Download `knulli-rex-brick-<version>.img.gz`, then flash it to a microSD card.

```bash
# macOS / Linux:
gunzip -c knulli-rex-brick-<version>.img.gz | sudo dd of=/dev/diskN bs=4M status=progress
```

Replace `/dev/diskN` with the SD card device. Double-check the target before running `dd`; writing to the wrong disk can destroy data.

You can also flash the `.img.gz` file directly with balenaEtcher.

### First boot

Insert the SD card into the Brick and boot it. First-time setup takes two boots and runs automatically:

1. On the first boot, Knulli expands the `userdata` partition to fill the SD card. The Rex boot hook then stages an ext4 conversion and reboots the device — do not power off when the Brick restarts on its own.
2. On the second boot, the hook formats `userdata` as ext4, copies the Rex payload from the boot partition, extracts the preconfigured Debian chroot, and installs the Knulli-side launcher files. Network access is not required.
3. Rex starts automatically (after a short delay for display initialization). Configure Wi-Fi from Rex's built-in Wi-Fi settings.
4. Future boots skip the setup work and start Rex directly.

After installation, EmulationStation is disabled at startup so Rex gets the screen on every boot; launch it from Rex's games tile when you want it. Rex is also installed as an EmulationStation fallback launcher under Tools → Rex, so you can launch Rex from ES for recovery if needed.

The Debian chroot tarball and dependencies are included in the image, so this path does not require a separate chroot download or first-boot package install.

First-boot logs are written to `/var/log/rex-postshare.log` on the device.

## Path 2: Tarball install over SSH

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

Besides the Rex binary and launcher files, the payload includes the host-side Wi-Fi daemon (`rex-wifid`) and bundled speech-to-text models (Parakeet by default), so voice dictation works without extra downloads.

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

## Path 3: Package upgrade

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

The `.deb` is a chroot-only binary upgrade. It does **not** update Knulli host files such as `/userdata/system/apps/rex/` (including the `rex-wifid` Wi-Fi daemon), `Rex.sh`, or `custom.sh`. To refresh those host-side files, rerun the tarball installer from Path 2 with the current release.

## Verify the install

After any install or upgrade, check the installed binary inside the chroot:

```bash
rex --version          # prints the build SHA
which rex              # should print /usr/bin/rex
```

Use `/userdata/system/apps/rex/debian-enter.sh` if you need to enter the Debian chroot from the Knulli host.

## Troubleshooting

### Rex does not start after the first image boot

Check `/var/log/rex-postshare.log` over SSH as `root`. The image install should not need Wi-Fi during first boot; failures usually mean the ext4 conversion did not complete, the bundled payload did not extract, the chroot did not unpack, or the launcher files were not installed. A failed install is retried on the next boot.

If EmulationStation is visible, check whether the fallback launcher exists at `/userdata/roms/tools/Rex.sh` and try launching Rex from Tools → Rex.

### Wi-Fi does not work inside Rex

Rex manages Wi-Fi through `rex-wifid`, a daemon that runs as root on the Knulli host and talks to ConnMan over D-Bus. Rex inside the chroot reaches it through a Unix socket at `/tmp/rex-wifi.sock`. If networks do not appear or password entry fails, reboot once and try again. If the problem persists, check over SSH as `root`:

```bash
pidof rex-wifid              # the daemon should be running
ls -l /tmp/rex-wifi.sock     # the socket should exist
tail /tmp/rex-launch.log     # daemon errors are logged here
```

`rex-wifid` is started by `/userdata/system/apps/rex/start-rex.sh`. If the binary is missing from `/userdata/system/apps/rex/`, rerun the tarball installer from Path 2.

### Package install fails with `dependency not satisfiable: tmux (>= 3.5)`

In the chroot, tmux 3.5a comes from Debian bookworm-backports. The installer normally configures this repository. If you skipped that setup, add the backports source manually:

```bash
echo 'deb http://deb.debian.org/debian bookworm-backports main' | sudo tee /etc/apt/sources.list.d/backports.list
sudo apt update
```

Then rerun the package install.

## See also

- [Rex architecture](https://github.com/xzys/rex/blob/master/ARCHITECTURE.md)
- [Rex releases](https://github.com/xzys/rex/releases)
