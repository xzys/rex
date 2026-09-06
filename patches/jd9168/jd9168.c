/*
 * jd9168 LCD panel driver for the Allwinner sun50iw10 (A133) disp2 BSP.
 *
 * DSI initialisation sequence extracted from the stock TrimUI Brick U-Boot with
 * scripts/brick/extract-uboot-panels.py, emitted by scripts/brick/mkpanel.py.
 * Regenerate rather than hand-editing the sequence:
 *
 *   ./scripts/brick/extract-uboot-panels.py u-boot.bin --at 0x9e88c > seq.txt
 *   ./scripts/brick/mkpanel.py seq.txt jd9168 > jd9168.c
 *
 * Panel: 1024x768, MIPI-DSI, 4 lanes, video mode, 53 MHz dclk.
 * Rails: lcd_power0 = cldo1, lcd_power1 = cldo4. Backlight: PWM channel 0.
 */
#include "jd9168.h"

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

/* initialisation payloads, recovered from stock U-Boot */
static u8 p00_DF[] = {
	0x91, 0x68, 0xF9,
};  /* password */
static u8 p01_B2[] = {
	0x00, 0x7E,
};
static u8 p02_B3[] = {
	0x00, 0x7E,
};
static u8 p03_C1[] = {
	0x00, 0x10, 0x00, 0x00, 0x00, 0x00,
};
static u8 p04_BB[] = {
	0x02, 0x24, 0x07, 0x61, 0x19, 0x44, 0x44,
};
static u8 p05_BE[] = {
	0x1A, 0xF2,
};
static u8 p06_C3[] = {
	0x10, 0x17, 0x5A, 0x17, 0x5A, 0x05, 0x05, 0x05, 0x05, 0x15, 0x15, 0x31,
	0x05, 0xDF,
};
static u8 p07_C4[] = {
	0x11, 0x80, 0x00, 0xDF, 0x09, 0x06, 0x14,
};
static u8 p08_CE[] = {
	0x00, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03,
	0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x0F, 0x03,
};
static u8 p09_CF[] = {
	0x00, 0x01, 0x40, 0x01, 0xCA, 0x01, 0xCA, 0x01, 0xCA,
};
static u8 p10_D0[] = {
	0x00, 0x1F, 0x1F, 0x1F, 0x1F, 0x1F, 0x1F, 0x1F, 0x1F, 0x1F, 0x1F, 0x03,
	0x01, 0x05, 0x07, 0x09, 0x0B, 0x1E, 0x15, 0x1F, 0x1F, 0x15, 0x1F,
};
static u8 p11_D1[] = {
	0x00, 0x1F, 0x1F, 0x1F, 0x1F, 0x1F, 0x1F, 0x1F, 0x1F, 0x1F, 0x1F, 0x02,
	0x00, 0x04, 0x06, 0x08, 0x0A, 0x1E, 0x15, 0x1F, 0x1F, 0x15, 0x1F,
};
static u8 p12_D2[] = {
	0x00, 0x1F, 0x1F, 0x1F, 0x1F, 0x1F, 0x1F, 0x1F, 0x1F, 0x1F, 0x1F, 0x00,
	0x02, 0x0A, 0x08, 0x06, 0x04, 0x1F, 0x15, 0x1F, 0x1F, 0x15, 0x1E,
};
static u8 p13_D3[] = {
	0x00, 0x1F, 0x1F, 0x1F, 0x1F, 0x1F, 0x1F, 0x1F, 0x1F, 0x1F, 0x1F, 0x01,
	0x03, 0x0B, 0x09, 0x07, 0x05, 0x1F, 0x15, 0x1F, 0x1F, 0x15, 0x1E,
};
static u8 p14_D4[] = {
	0x30, 0x00, 0x00, 0x04, 0x00, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03,
	0x03, 0x00, 0x11, 0x00, 0x01, 0xC0, 0x04, 0x01, 0x01, 0x11, 0x80, 0x01,
	0xC0, 0x05, 0x01, 0x01, 0x00, 0x00, 0x00, 0x04, 0x00, 0x06, 0x18, 0x02,
	0xE3,
};
static u8 p15_D5[] = {
	0x68, 0x73, 0x00, 0x08, 0x08, 0x00, 0x03, 0x00,
};
static u8 p16_B7[] = {
	0x00, 0xD8, 0x00, 0x00, 0xD8, 0x00,
};
static u8 p17_C8[] = {
	0x7F, 0x69, 0x5A, 0x4E, 0x4A, 0x3B, 0x40, 0x2A, 0x44, 0x43, 0x44, 0x63,
	0x51, 0x59, 0x4C, 0x48, 0x3A, 0x28, 0x0F, 0x7F, 0x69, 0x5A, 0x4E, 0x4A,
	0x3B, 0x40, 0x2A, 0x44, 0x43, 0x44, 0x63, 0x51, 0x59, 0x4C, 0x48, 0x3A,
	0x28, 0x0F,
};
static u8 p18_BB[] = {
	0x00, 0x5B, 0x5C, 0x41,
};
static u8 p19_B5[] = {
	0x00, 0x5A, 0x0A,
};

static void LCD_panel_init(u32 sel)
{
	sunxi_lcd_dsi_clk_enable(sel);
	sunxi_lcd_delay_ms(10);

	sunxi_lcd_dsi_dcs_write(sel, 0xDF, p00_DF, 3);
	sunxi_lcd_dsi_dcs_write_1para(sel, 0xDE, 0x00);  /* page select */
	sunxi_lcd_dsi_dcs_write(sel, 0xB2, p01_B2, 2);
	sunxi_lcd_dsi_dcs_write(sel, 0xB3, p02_B3, 2);
	sunxi_lcd_dsi_dcs_write(sel, 0xC1, p03_C1, 6);
	sunxi_lcd_dsi_dcs_write(sel, 0xBB, p04_BB, 7);
	sunxi_lcd_dsi_dcs_write(sel, 0xBE, p05_BE, 2);
	sunxi_lcd_dsi_dcs_write(sel, 0xC3, p06_C3, 14);
	sunxi_lcd_dsi_dcs_write(sel, 0xC4, p07_C4, 7);
	sunxi_lcd_dsi_dcs_write(sel, 0xCE, p08_CE, 23);
	sunxi_lcd_dsi_dcs_write(sel, 0xCF, p09_CF, 9);
	sunxi_lcd_dsi_dcs_write(sel, 0xD0, p10_D0, 23);
	sunxi_lcd_dsi_dcs_write(sel, 0xD1, p11_D1, 23);
	sunxi_lcd_dsi_dcs_write(sel, 0xD2, p12_D2, 23);
	sunxi_lcd_dsi_dcs_write(sel, 0xD3, p13_D3, 23);
	sunxi_lcd_dsi_dcs_write(sel, 0xD4, p14_D4, 37);
	sunxi_lcd_dsi_dcs_write(sel, 0xD5, p15_D5, 8);
	sunxi_lcd_dsi_dcs_write(sel, 0xB7, p16_B7, 6);
	sunxi_lcd_dsi_dcs_write(sel, 0xC8, p17_C8, 38);
	sunxi_lcd_dsi_dcs_write_1para(sel, 0xDE, 0x02);  /* page select */
	sunxi_lcd_dsi_dcs_write(sel, 0xBB, p18_BB, 4);
	sunxi_lcd_dsi_dcs_write(sel, 0xB5, p19_B5, 3);
	sunxi_lcd_dsi_dcs_write_1para(sel, 0xC6, 0x22);
	sunxi_lcd_dsi_dcs_write_1para(sel, 0xD7, 0x12);
	sunxi_lcd_dsi_dcs_write_1para(sel, 0xDE, 0x00);  /* page select */
	sunxi_lcd_dsi_dcs_write_1para(sel, 0x35, 0x00);
	sunxi_lcd_dsi_dcs_write_0para(sel, 0x11);  /* SLPOUT */
	sunxi_lcd_delay_ms(120);
	sunxi_lcd_dsi_dcs_write_0para(sel, 0x29);  /* DSPON */
	sunxi_lcd_delay_ms(50);
}

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

struct __lcd_panel jd9168_panel = {
	.name = "jd9168",
	.func = {
		.cfg_panel_info = LCD_cfg_panel_info,
		.cfg_open_flow = LCD_open_flow,
		.cfg_close_flow = LCD_close_flow,
		.lcd_user_defined_func = LCD_user_defined_func,
	},
};
