# `jd9168` panel driver for the A133 (sun50iw10) disp2 BSP

The thing that blocked a native Linux kernel on the TrimUI Brick since March 2026: the
kernel had no driver for the Brick's LCD, so it could not initialise the panel, and any
attempt to take over the display killed the DSI link that U-Boot had left working.

`jd9168.c` here is generated, not hand-written. Regenerate it rather than editing it:

```bash
./scripts/brick/extract-uboot-panels.py <u-boot.bin> --list          # find the tables
./scripts/brick/extract-uboot-panels.py <u-boot.bin> --at 0x9e88c \
    > patches/jd9168/jd9168-seq.txt
./scripts/brick/mkpanel.py patches/jd9168/jd9168-seq.txt jd9168 > patches/jd9168/jd9168.c
```

## Where the sequence came from

TrimUI's U-Boot carries init tables for **seven** panels and picks one at runtime by
reading the panel's RDID (DCS `0xDA`/`0xDB`/`0xDC`) — hence the strings `check_lcd_module`,
`LCD module jd9168`, `LCD module otm1289a`, `LCD module st7703`, `nv3052`, `gc9702c`,
`er68576` in the binary. The Brick answers `91 68 03`, which is **Jadard JD9168**. That is
also why the device tree says `otm1289a` while the cmdline says `lcd=jd9168`: `otm1289a`
is the name of the multiplexer, not of this panel.

The JD9168 table lives at file offset **`0x9e88c`**, 30 records, and opens with the
JD9168 unlock `DF 91 68 F9`.

Source binary: `u-boot.bin`, 786,432 bytes, sha256
`5e96fa7ef5a5d6f0e10efa5d60a7affa7cbf73eb1a8679bcf7efb471e52a0437`, from
<https://github.com/knulli-cfw/knulli-linux/tree/knulli-main/package/boot/uboot-a133/trimui-brick/boot_package>.
Knulli ships it as a binary extracted from TrimUI's stock firmware, since TrimUI has
never released source.

## Applying it

Copy both files into `drivers/video/fbdev/sunxi/disp2/disp/lcd/` in the BSP kernel
(`chainsx/kernel-sun50iw10` branch `4.9`), then three registration edits:

1. `disp2/disp/lcd/panels.c` — add `&jd9168_panel,` to `panel_array[]`
2. `disp2/disp/lcd/panels.h` — add `extern struct __lcd_panel jd9168_panel;`
3. `disp2/disp/Makefile` — add `jd9168.o` to `disp-objs`

The name must stay `"jd9168"` so it matches `lcd=jd9168` on the U-Boot cmdline.

## Not yet tested on hardware

Generated and compile-checked only; the Brick was not reachable when this was written.
Specifically unverified:

- **Power-on and reset timing.** `LCD_power_on()` uses the DTB's rail order
  (`lcd_power0` = cldo1, `lcd_power1` = cldo4) and a plausible reset pulse on
  `lcd_gpio_0`, but the stock timings were not recovered — they live in U-Boot's code, not
  in the table. If the panel does not come up, this is the first thing to instrument.
- **Whether re-initialising is even wanted.** If `notes/brick-display-repoint-design.md`
  Approach A works, the display can be taken over *without* re-initialising the panel, and
  this driver is only needed for suspend/resume and a clean modeset.
- The gamma table in `LCD_cfg_panel_info()` is the BSP's generic linear ramp, not the
  panel's own.
