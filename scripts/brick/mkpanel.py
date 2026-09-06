#!/usr/bin/env python3
"""Turn a recovered DSI init sequence into a compilable sunxi BSP panel driver.

Last mile of the reverse-engineering path: FindDsiCadence.java recovers the
(register, value) pairs from the stock kernel; this emits the jd9168.c that the
4.9.191 BSP expects, modelled on disp2/disp/lcd/k101im2qa04.c.

Input: whitespace-separated hex pairs, one per line, "RR VV".
  - a single value on a line  -> 0-parameter DCS write (SLPOUT 0x11, DSPON 0x29)
  - "delay N"                 -> explicit delay in ms
  - '#' starts a comment

    ./mkpanel.py pairs.txt jd9168 > jd9168.c
    ./mkpanel.py pairs.txt jd9168 --header > jd9168.h

Then three registration edits in the BSP tree:
  1. disp2/disp/lcd/panels.c -- add &jd9168_panel to panel_array[]
  2. disp2/disp/lcd/panels.h -- extern struct __lcd_panel jd9168_panel;
  3. disp2/disp/Makefile     -- add jd9168.o to disp-objs
"""
import sys

HEAD = r"""/*
 * %(name)s LCD panel driver for the Allwinner sun50iw10 (A133) disp2 BSP.
 *
 * DSI initialisation sequence recovered from the stock TrimUI Brick kernel with
 * scripts/ghidra/FindDsiCadence.java, emitted by scripts/brick/mkpanel.py.
 * Regenerate rather than hand-editing the sequence.
 *
 * Panel: 1024x768, MIPI-DSI, 4 lanes, video mode, 53 MHz dclk.
 * Rails: lcd_power0 = cldo1, lcd_power1 = cldo4. Backlight: PWM channel 0.
 */
#include "%(name)s.h"

static void LCD_power_on(u32 sel);
static void LCD_power_off(u32 sel);
static void LCD_bl_open(u32 sel);
static void LCD_bl_close(u32 sel);
static void LCD_panel_init(u32 sel);
static void LCD_panel_exit(u32 sel);

static void LCD_cfg_panel_info(struct panel_extend_para *info)
{
	u32 i = 0, j = 0;
	u32 items;
	u8 lcd_gamma_tbl[][2] = {
		{0, 0}, {15, 15}, {30, 30}, {45, 45}, {60, 60}, {75, 75},
		{90, 90}, {105, 105}, {120, 120}, {135, 135}, {150, 150},
		{165, 165}, {180, 180}, {195, 195}, {210, 210}, {225, 225},
		{240, 240}, {255, 255},
	};

	items = sizeof(lcd_gamma_tbl) / 2;
	for (i = 0; i < items - 1; i++) {
		u32 num = lcd_gamma_tbl[i + 1][0] - lcd_gamma_tbl[i][0];

		for (j = 0; j < num; j++) {
			u32 value = lcd_gamma_tbl[i][1]
				+ ((lcd_gamma_tbl[i + 1][1] - lcd_gamma_tbl[i][1]) * j) / num;
			info->lcd_gamma_tbl[lcd_gamma_tbl[i][0] + j] =
				(value << 16) + (value << 8) + value;
		}
	}
	info->lcd_gamma_tbl[255] = (lcd_gamma_tbl[items - 1][1] << 16)
		+ (lcd_gamma_tbl[items - 1][1] << 8) + lcd_gamma_tbl[items - 1][1];
}

static s32 LCD_open_flow(u32 sel)
{
	LCD_OPEN_FUNC(sel, LCD_power_on, 50);
	LCD_OPEN_FUNC(sel, LCD_panel_init, 50);
	LCD_OPEN_FUNC(sel, sunxi_lcd_tcon_enable, 100);
	LCD_OPEN_FUNC(sel, LCD_bl_open, 0);
	return 0;
}

static s32 LCD_close_flow(u32 sel)
{
	LCD_CLOSE_FUNC(sel, LCD_bl_close, 0);
	LCD_CLOSE_FUNC(sel, sunxi_lcd_tcon_disable, 0);
	LCD_CLOSE_FUNC(sel, LCD_panel_exit, 100);
	LCD_CLOSE_FUNC(sel, LCD_power_off, 0);
	return 0;
}

static void LCD_power_on(u32 sel)
{
	sunxi_lcd_pin_cfg(sel, 1);
	sunxi_lcd_power_enable(sel, 0);   /* lcd_power0: cldo1 */
	sunxi_lcd_delay_ms(5);
	sunxi_lcd_power_enable(sel, 1);   /* lcd_power1: cldo4 */
	sunxi_lcd_delay_ms(20);
	/* reset pulse on lcd_gpio_0 */
	sunxi_lcd_gpio_set_value(sel, 0, 1);
	sunxi_lcd_delay_ms(10);
	sunxi_lcd_gpio_set_value(sel, 0, 0);
	sunxi_lcd_delay_ms(10);
	sunxi_lcd_gpio_set_value(sel, 0, 1);
	sunxi_lcd_delay_ms(20);
}

static void LCD_power_off(u32 sel)
{
	sunxi_lcd_gpio_set_value(sel, 0, 0);
	sunxi_lcd_delay_ms(5);
	sunxi_lcd_power_disable(sel, 1);
	sunxi_lcd_delay_ms(5);
	sunxi_lcd_power_disable(sel, 0);
	sunxi_lcd_pin_cfg(sel, 0);
}

static void LCD_bl_open(u32 sel)
{
	sunxi_lcd_pwm_enable(sel);
	sunxi_lcd_backlight_enable(sel);
}

static void LCD_bl_close(u32 sel)
{
	sunxi_lcd_backlight_disable(sel);
	sunxi_lcd_pwm_disable(sel);
}

static void LCD_panel_init(u32 sel)
{
	sunxi_lcd_dsi_clk_enable(sel);
	sunxi_lcd_delay_ms(10);

"""

TAIL = r"""}

static void LCD_panel_exit(u32 sel)
{
	sunxi_lcd_dsi_dcs_write_0para(sel, 0x28);  /* DSPOFF */
	sunxi_lcd_delay_ms(20);
	sunxi_lcd_dsi_dcs_write_0para(sel, 0x10);  /* SLPIN  */
	sunxi_lcd_delay_ms(80);
}

static s32 LCD_user_defined_func(u32 sel, u32 para1, u32 para2, u32 para3)
{
	return 0;
}

struct __lcd_panel %(name)s_panel = {
	.name = "%(name)s",
	.func = {
		.cfg_panel_info = LCD_cfg_panel_info,
		.cfg_open_flow = LCD_open_flow,
		.cfg_close_flow = LCD_close_flow,
		.lcd_user_defined_func = LCD_user_defined_func,
	},
};
"""

HEADER = r"""#ifndef _%(upper)s_H
#define _%(upper)s_H

#include "panels.h"

extern struct __lcd_panel %(name)s_panel;

#endif
"""


def parse(path):
    ops = []
    for raw in open(path):
        line = raw.split("#")[0].strip()
        if not line:
            continue
        parts = line.replace(",", " ").split()
        if parts[0].lower() == "delay":
            ops.append(("delay", int(parts[1], 0)))
        elif len(parts) == 1:
            ops.append(("dcs0", int(parts[0], 16)))
        else:
            ops.append(("dcs1", int(parts[0], 16), int(parts[1], 16)))
    return ops


def main():
    if len(sys.argv) < 3:
        print(__doc__)
        return 1
    name = sys.argv[2]
    ctx = {"name": name, "upper": name.upper()}
    if "--header" in sys.argv:
        sys.stdout.write(HEADER % ctx)
        return 0

    ops = parse(sys.argv[1])
    out = [HEAD % ctx]
    n = 0
    for op in ops:
        if op[0] == "dcs1":
            out.append("\tsunxi_lcd_dsi_dcs_write_1para(sel, 0x%02X, 0x%02X);\n" % (op[1], op[2]))
            n += 1
        elif op[0] == "dcs0":
            cmt = {0x11: "  /* SLPOUT */", 0x29: "  /* DSPON */"}.get(op[1], "")
            out.append("\tsunxi_lcd_dsi_dcs_write_0para(sel, 0x%02X);%s\n" % (op[1], cmt))
            n += 1
        else:
            out.append("\tsunxi_lcd_delay_ms(%d);\n" % op[1])
    out.append(TAIL % ctx)
    sys.stdout.write("".join(out))
    sys.stderr.write("emitted %d DCS operations for %s\n" % (n, name))
    return 0


if __name__ == "__main__":
    sys.exit(main())
