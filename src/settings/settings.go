package settings

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	cfg "github.com/xrfr/t-rex/v2/config"
	"github.com/xrfr/t-rex/v2/wifi"
)

type screenKind int

const (
	screenSettings screenKind = iota
	screenWifi
)

type rowKind int

const (
	rowHeader rowKind = iota
	rowValue
)

type row struct {
	kind  rowKind
	id    string // value-row identifier; ignored for headers
	label string
}

// Section + value-row identifiers. Order in this list = display order.
const (
	idKeyboardMode   = "keyboard_mode"
	idKeyboardLayout = "keyboard_layout"
	idKeyboardView   = "keyboard_view"
	idDefaultVoice   = "default_voice"
	idSTTMode        = "stt_mode"
	idMenuGap        = "menu_gap"
	idIconSize       = "icon_size"
	idBrightness     = "brightness"
	idRexOnBoot      = "rex_on_boot"
	idTimezone       = "timezone"
	idWifiOn         = "wifi_on"
	idWifi           = "wifi"
)

// Bounds for the numeric menu-layout rows. Step 1 in each direction with
// clamping (no wrap), since extreme values would render an unusable grid.
const (
	menuGapMin = 0
	menuGapMax = 12
	iconSizeMin   = 6
	iconSizeMax   = 16
)

// Brightness adjusts in 10% steps.
const (
	brightnessStep = 10
	brightnessMin  = 0
	brightnessMax  = 100
)

type wifiPoweredMsg struct {
	on  bool
	err error
}

// settingsTickMsg drives a periodic re-query of live system state (currently
// the WiFi power row) so the screen reflects changes that happen out of band
// — e.g. ConnMan settling a power toggle asynchronously, or WiFi being flipped
// elsewhere.
type settingsTickMsg time.Time

func settingsTick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return settingsTickMsg(t) })
}

type Model struct {
	config        cfg.Config
	rows          []row
	cursor        int // index into rows; always lands on a rowValue
	width         int
	height        int
	kbModes       []string
	kbModeIndex   int
	kbLayouts     []string
	kbLayoutIndex int
	kbViews       []string
	kbViewIndex   int
	sttModes      []string
	sttModeIndex  int
	timezones     []string
	tzIndex       int
	defaultVoice  bool
	rexOnBoot     bool
	status        string
	screen        screenKind
	wifiClient    *wifi.Client
	wifiModel     *wifi.Model
	wifiPowered   *bool // nil = not yet queried
	backlight     *backlight
	brightnessPct int    // 0..100, only meaningful when backlight != nil
	prevWindow    string // tmux window name before entering wifi sub-screen
}

// backlight is the discovered LCD brightness backend. Two backends are probed:
//
//   - Standard LED backlight class: /sys/class/backlight/<dev>/{brightness,
//     max_brightness}. Writes go through `sudo tee` (root-owned on R36S and
//     most mainline kernels).
//   - TrimUI Brick: the vendor /dev/disp ioctl, reached by shelling out to a
//     copy of /usr/bin/brightness staged at /userdata/system/apps/rex/brightness
//     (Rex.sh copies it from the host on each launch — the host binary lives
//     outside the Debian chroot but /userdata is bind-mounted in). The sysfs
//     entry /sys/class/led/tg5040_exterme_brightness looks tempting but is a
//     memo only — writes there do not change the panel.
type backlight struct {
	file string // sysfs path, set for the LED-class backend
	max  int    // raw range upper bound for both backends
	cmd  string // brightness binary path, set for the brick backend
}

const brickBrightnessBin = "/userdata/system/apps/rex/brightness"

func discoverBacklight() *backlight {
	if entries, err := os.ReadDir("/sys/class/backlight"); err == nil {
		for _, e := range entries {
			p := filepath.Join("/sys/class/backlight", e.Name())
			raw, err := os.ReadFile(filepath.Join(p, "max_brightness"))
			if err != nil {
				continue
			}
			max, err := strconv.Atoi(strings.TrimSpace(string(raw)))
			if err != nil || max <= 0 {
				continue
			}
			return &backlight{file: filepath.Join(p, "brightness"), max: max}
		}
	}
	if _, err := os.Stat("/dev/disp"); err == nil {
		if _, err := os.Stat(brickBrightnessBin); err == nil {
			return &backlight{cmd: brickBrightnessBin, max: 255}
		}
	}
	return nil
}

func (b *backlight) readPercent() int {
	var raw string
	if b.cmd != "" {
		out, err := exec.Command("sudo", "-n", b.cmd, "get").Output()
		if err != nil {
			return brightnessMax
		}
		raw = string(out)
	} else {
		bs, err := os.ReadFile(b.file)
		if err != nil {
			return brightnessMax
		}
		raw = string(bs)
	}
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return brightnessMax
	}
	pct := v * 100 / b.max
	return clamp(pct, brightnessMin, brightnessMax)
}

func (b *backlight) writePercent(pct int) error {
	raw := b.max * pct / 100
	if b.cmd != "" {
		out, err := exec.Command("sudo", "-n", b.cmd, "set", strconv.Itoa(raw)).CombinedOutput()
		if err != nil {
			return fmt.Errorf("set brightness: %v: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	cmd := exec.Command("sudo", "-n", "tee", b.file)
	cmd.Stdin = strings.NewReader(strconv.Itoa(raw) + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("set brightness: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func Run() error {
	config, err := cfg.LoadConfig()
	if err != nil {
		return err
	}

	m := NewModel(config)
	defer m.closeWifi()
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func (m *Model) closeWifi() {
	m.wifiModel = nil
	if m.wifiClient != nil {
		_ = m.wifiClient.Close()
		m.wifiClient = nil
	}
}

// ensureWifiClient lazily opens a single ConnMan dbus connection shared by
// the WiFi power toggle and the wifi sub-screen. Returns nil + sets status
// on failure (e.g. dbus unavailable).
func (m *Model) ensureWifiClient() *wifi.Client {
	if m.wifiClient != nil {
		return m.wifiClient
	}
	c, err := wifi.NewClient()
	if err != nil {
		m.status = "wifi: " + err.Error()
		return nil
	}
	m.wifiClient = c
	return c
}

func (m *Model) queryWifiPowered() tea.Cmd {
	c := m.ensureWifiClient()
	if c == nil {
		return nil
	}
	return func() tea.Msg {
		on, err := c.WifiPowered()
		return wifiPoweredMsg{on: on, err: err}
	}
}

func (m *Model) toggleWifiPowered() tea.Cmd {
	c := m.ensureWifiClient()
	if c == nil {
		return nil
	}
	want := true
	if m.wifiPowered != nil {
		want = !*m.wifiPowered
	}
	return func() tea.Msg {
		if err := c.SetWifiPowered(want); err != nil {
			return wifiPoweredMsg{err: err}
		}
		// Re-query after the change so the displayed state reflects what
		// ConnMan actually settled on (it may reject e.g. when rfkill blocks).
		on, err := c.WifiPowered()
		return wifiPoweredMsg{on: on, err: err}
	}
}

func NewModel(config cfg.Config) *Model {
	kbModes := cfg.KeyboardModes()
	kbLayouts := cfg.KeyboardLayouts()
	kbViews := cfg.KeyboardViews()
	sttModes := cfg.STTModes()
	timezones := cfg.Timezones()

	m := &Model{
		config:        config,
		kbModes:       kbModes,
		kbModeIndex:   indexOf(kbModes, config.KeyboardMode),
		kbLayouts:     kbLayouts,
		kbLayoutIndex: indexOf(kbLayouts, cfg.NormalizeKeyboardLayout(config.KeyboardLayout)),
		kbViews:       kbViews,
		kbViewIndex:   indexOf(kbViews, cfg.NormalizeKeyboardView(config.KeyboardView)),
		sttModes:      sttModes,
		sttModeIndex:  indexOf(sttModes, cfg.NormalizeSTTMode(config.STTMode)),
		timezones:     timezones,
		tzIndex:       indexOf(timezones, cfg.NormalizeTimezone(config.Timezone)),
		defaultVoice:  config.DefaultVoice,
		rexOnBoot:     rexEnabledOnBoot(),
		backlight:     discoverBacklight(),
	}
	if m.backlight != nil {
		m.brightnessPct = m.backlight.readPercent()
	}
	m.rows = []row{
		{kind: rowHeader, label: "Keyboard"},
		{kind: rowValue, id: idKeyboardMode, label: "Chording mode"},
		{kind: rowValue, id: idKeyboardLayout, label: "Keyboard layout"},
		{kind: rowValue, id: idKeyboardView, label: "Keyboard view"},
		{kind: rowValue, id: idDefaultVoice, label: "Default mode"},
		{kind: rowValue, id: idSTTMode, label: "STT mode"},
		{kind: rowHeader, label: "Menu"},
		{kind: rowValue, id: idMenuGap, label: "Gap"},
		{kind: rowValue, id: idIconSize, label: "Icon size"},
		{kind: rowHeader, label: "System Settings"},
		{kind: rowValue, id: idBrightness, label: "Brightness"},
		{kind: rowValue, id: idRexOnBoot, label: "Start Rex on boot"},
		{kind: rowValue, id: idTimezone, label: "Timezone"},
		{kind: rowValue, id: idWifiOn, label: "Wifi On"},
		{kind: rowValue, id: idWifi, label: "Wifi Network"},
	}
	// Cursor starts on the first value row.
	for i, r := range m.rows {
		if r.kind == rowValue {
			m.cursor = i
			break
		}
	}
	return m
}

func rexEnabledOnBoot() bool {
	// On Knulli/Brick there is no systemd — Rex always autostarts
	// via /userdata/system/custom.sh.
	if _, err := exec.LookPath("systemctl"); err != nil {
		return true
	}
	return exec.Command("systemctl", "is-enabled", "--quiet", "rex.service").Run() == nil
}

func setRexOnBoot(enable bool) error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("rex autostart is always on (no systemd on this device)")
	}
	if enable {
		if out, err := exec.Command("sudo", "systemctl", "enable", "rex.service").CombinedOutput(); err != nil {
			return fmt.Errorf("enable rex: %v: %s", err, strings.TrimSpace(string(out)))
		}
		if out, err := exec.Command("sudo", "systemctl", "disable", "emulationstation.service").CombinedOutput(); err != nil {
			return fmt.Errorf("disable emulationstation: %v: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	if out, err := exec.Command("sudo", "systemctl", "disable", "rex.service").CombinedOutput(); err != nil {
		return fmt.Errorf("disable rex: %v: %s", err, strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("sudo", "systemctl", "enable", "emulationstation.service").CombinedOutput(); err != nil {
		return fmt.Errorf("enable emulationstation: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// applyTimezone repoints /etc/localtime at the chosen IANA zone and updates
// /etc/timezone. glibc rereads /etc/localtime per-call, so the tmux clock
// updates without restarting anything. Skipped in unit tests via REX_SKIP_TZ_APPLY.
func applyTimezone(tz string) error {
	if os.Getenv("REX_SKIP_TZ_APPLY") == "1" {
		return nil
	}
	src := "/usr/share/zoneinfo/" + tz
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("zoneinfo missing for %s", tz)
	}
	if out, err := exec.Command("sudo", "ln", "-sf", src, "/etc/localtime").CombinedOutput(); err != nil {
		return fmt.Errorf("set localtime: %v: %s", err, strings.TrimSpace(string(out)))
	}
	cmd := exec.Command("sudo", "tee", "/etc/timezone")
	cmd.Stdin = strings.NewReader(tz + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("set timezone file: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func indexOf(values []string, target string) int {
	for i, v := range values {
		if v == target {
			return i
		}
	}
	return 0
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.queryWifiPowered(), settingsTick())
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if sz, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = sz.Width
		m.height = sz.Height
		// Forward to the wifi submodel too so its View renders at the right
		// size whether or not it's currently on-screen.
		if m.wifiModel != nil {
			nm, _ := m.wifiModel.Update(msg)
			if w, ok := nm.(*wifi.Model); ok {
				m.wifiModel = w
			}
		}
		return m, nil
	}

	if _, ok := msg.(settingsTickMsg); ok {
		// Re-query WiFi power while on the settings list so the row self-
		// corrects after a toggle ConnMan applied asynchronously. The wifi
		// sub-screen runs its own refresh loop, so skip there. Always re-arm
		// the tick so it resumes when we return to the list.
		var q tea.Cmd
		if m.screen == screenSettings && m.wifiClient != nil {
			q = m.queryWifiPowered()
		}
		return m, tea.Batch(q, settingsTick())
	}

	if pm, ok := msg.(wifiPoweredMsg); ok {
		if pm.err != nil {
			switch {
			case errors.Is(pm.err, wifi.ErrNotAvailable):
				m.status = "WiFi unavailable"
			case errors.Is(pm.err, wifi.ErrBlocked):
				m.status = "WiFi blocked (check D-Bus config)"
			default:
				m.status = "wifi: " + pm.err.Error()
			}
		} else {
			on := pm.on
			m.wifiPowered = &on
		}
		return m, nil
	}

	if m.screen == screenWifi {
		return m.updateWifi(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc", "b":
			return m, tea.Quit
		case "up", "k":
			m.moveCursor(-1)
		case "down", "j":
			m.moveCursor(1)
		case "left", "h":
			if r := m.rows[m.cursor]; r.kind == rowValue && r.id == idWifiOn {
				return m, m.toggleWifiPowered()
			}
			m.cycleMode(-1)
		case "right", "l", "enter", "a":
			if r := m.rows[m.cursor]; r.kind == rowValue {
				switch r.id {
				case idWifi:
					return m, m.openWifi()
				case idWifiOn:
					return m, m.toggleWifiPowered()
				}
			}
			m.cycleMode(1)
		}
	}
	return m, nil
}

// updateWifi routes input to the wifi submodel while on the wifi sub-screen.
// esc/q/b return to the settings list (instead of quitting); ctrl+c still
// quits the whole program. Other keys (incl. wifi's own bindings — r, d, x,
// enter) pass through unchanged.
func (m *Model) updateWifi(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc", "q", "b":
			m.leaveWifi()
			return m, nil
		}
	}
	if m.wifiModel == nil {
		return m, nil
	}
	nm, cmd := m.wifiModel.Update(msg)
	if w, ok := nm.(*wifi.Model); ok {
		m.wifiModel = w
	}
	return m, cmd
}

// openWifi lazily constructs the wifi submodel (reusing the shared dbus
// client) and switches to the wifi screen. On dbus failure it surfaces the
// error in the status line and stays on the settings screen.
func (m *Model) openWifi() tea.Cmd {
	if m.wifiModel == nil {
		c := m.ensureWifiClient()
		if c == nil {
			return nil
		}
		m.wifiModel = wifi.NewModel(c)
		// Seed the submodel with the current window size so its first render
		// has correct dimensions before any subsequent WindowSizeMsg.
		if m.width > 0 || m.height > 0 {
			nm, _ := m.wifiModel.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
			if w, ok := nm.(*wifi.Model); ok {
				m.wifiModel = w
			}
		}
	}
	// Rename the tmux window to "wifi" so the wifi.js plugin engages and
	// gamepad Y/X/R3 map to F5/F4/F6 (refresh/disconnect/forget). Save the
	// previous name so we can restore it on exit.
	if prev, err := getTmuxWindowName(); err == nil {
		m.prevWindow = prev
	}
	_ = exec.Command("tmux", "rename-window", "wifi").Run()
	m.screen = screenWifi
	return m.wifiModel.Init()
}

// leaveWifi restores the tmux window name to whatever it was before the wifi
// sub-screen was opened, then returns to the settings list.
func (m *Model) leaveWifi() {
	if m.prevWindow != "" {
		_ = exec.Command("tmux", "rename-window", m.prevWindow).Run()
		m.prevWindow = ""
	}
	m.screen = screenSettings
}

func getTmuxWindowName() (string, error) {
	if os.Getenv("TMUX") == "" {
		return "", fmt.Errorf("not in tmux")
	}
	out, err := exec.Command("tmux", "display-message", "-p", "#{window_name}").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// moveCursor advances by `direction`, skipping header rows and wrapping
// around at either end (up at top → last value row, down at bottom → first).
func (m *Model) moveCursor(direction int) {
	n := len(m.rows)
	if n == 0 {
		return
	}
	i := m.cursor
	for step := 0; step < n; step++ {
		i = (i + direction + n) % n
		if m.rows[i].kind == rowValue {
			m.cursor = i
			return
		}
	}
}

func (m *Model) View() string {
	if m.screen == screenWifi && m.wifiModel != nil {
		return m.wifiModel.View()
	}
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	rowStyle := lipgloss.NewStyle()
	selectedRowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("4"))
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))

	viewWidth := m.width
	if viewWidth < 40 {
		viewWidth = 40
	}
	viewHeight := m.height
	if viewHeight < 8 {
		viewHeight = 8
	}

	borderLines := 2
	horizontalPadding := 4 // Padding(1, 2)
	verticalPadding := 2   // Padding(1, 2)

	innerWidth := viewWidth - borderLines - horizontalPadding
	if innerWidth < 20 {
		innerWidth = 20
	}

	// Render each settings row to a line (and a blank separator above each
	// section header). Track the line index that holds the cursor so we can
	// scroll to keep it visible when the list outgrows the viewport.
	var rowLines []string
	cursorLine := -1
	for i, r := range m.rows {
		if r.kind == rowHeader {
			if i > 0 {
				rowLines = append(rowLines, "")
			}
			rowLines = append(rowLines, sectionStyle.Render(r.label))
			continue
		}
		line := m.formatRow(r.label, m.valueFor(r.id), innerWidth)
		if m.cursor == i {
			line = selectedRowStyle.Render(line)
			cursorLine = len(rowLines)
		} else {
			line = rowStyle.Render(line)
		}
		rowLines = append(rowLines, line)
	}

	titleLines := 2 // "Settings" + blank
	helpLines := 1
	statusBlock := 0
	if m.status != "" {
		statusBlock = 2 // blank + status line
	}
	available := viewHeight - borderLines - verticalPadding - titleLines - helpLines - statusBlock
	if available < 1 {
		available = 1
	}

	// Pick a window of rowLines that contains the cursor. Center it when
	// possible, then clamp so we don't scroll past either end.
	start := 0
	if len(rowLines) > available && cursorLine >= 0 {
		start = cursorLine - available/2
		if start < 0 {
			start = 0
		}
		if start+available > len(rowLines) {
			start = len(rowLines) - available
		}
	}
	end := start + available
	if end > len(rowLines) {
		end = len(rowLines)
	}
	visible := rowLines[start:end]

	help := hintStyle.Render("↑/↓ select row • ←/→ change • esc back")

	var content strings.Builder
	content.WriteString(titleStyle.Render("Settings"))
	content.WriteString("\n\n")
	for i, l := range visible {
		if i > 0 {
			content.WriteString("\n")
		}
		content.WriteString(l)
	}
	for i := len(visible); i < available; i++ {
		content.WriteString("\n")
	}
	if m.status != "" {
		content.WriteString("\n\n")
		content.WriteString(statusStyle.Render(m.status))
	}
	content.WriteString("\n")
	content.WriteString(help)

	box := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("4")).
		Padding(1, 2).
		Render(content.String())

	return box
}

func (m *Model) valueFor(id string) string {
	switch id {
	case idKeyboardMode:
		return m.kbModes[m.kbModeIndex]
	case idKeyboardLayout:
		return m.kbLayouts[m.kbLayoutIndex]
	case idKeyboardView:
		return m.kbViews[m.kbViewIndex]
	case idDefaultVoice:
		return defaultVoiceLabel(m.defaultVoice, m.config.DefaultVoiceSticky)
	case idSTTMode:
		return m.sttModes[m.sttModeIndex]
	case idMenuGap:
		return fmt.Sprintf("%d", m.config.Layout.MarginX)
	case idIconSize:
		return fmt.Sprintf("%d", m.config.Layout.SquareSize)
	case idBrightness:
		if m.backlight == nil {
			return "n/a"
		}
		return fmt.Sprintf("%d%%", m.brightnessPct)
	case idRexOnBoot:
		return onOff(m.rexOnBoot)
	case idTimezone:
		return m.timezones[m.tzIndex]
	case idWifiOn:
		if m.wifiPowered == nil {
			return "…"
		}
		return onOff(*m.wifiPowered)
	case idWifi:
		return "→"
	}
	return ""
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// defaultVoiceLabel renders the tri-state for the "Default mode" row.
// sticky means: persist the voice/text state across keyboard launches (the
// keyboard rewrites DefaultVoice whenever the user toggles voice via Fn/MIC).
func defaultVoiceLabel(defaultVoice, sticky bool) string {
	if sticky {
		return "sticky"
	}
	if defaultVoice {
		return "voice"
	}
	return "qwerty"
}

func (m *Model) cycleMode(direction int) {
	r := m.rows[m.cursor]
	if r.kind != rowValue {
		return
	}
	switch r.id {
	case idKeyboardMode:
		if n := len(m.kbModes); n > 0 {
			m.kbModeIndex = (m.kbModeIndex + direction + n) % n
			m.config.KeyboardMode = m.kbModes[m.kbModeIndex]
		}
	case idKeyboardLayout:
		if n := len(m.kbLayouts); n > 0 {
			m.kbLayoutIndex = (m.kbLayoutIndex + direction + n) % n
			m.config.KeyboardLayout = m.kbLayouts[m.kbLayoutIndex]
		}
	case idKeyboardView:
		if n := len(m.kbViews); n > 0 {
			m.kbViewIndex = (m.kbViewIndex + direction + n) % n
			m.config.KeyboardView = m.kbViews[m.kbViewIndex]
		}
	case idSTTMode:
		if n := len(m.sttModes); n > 0 {
			m.sttModeIndex = (m.sttModeIndex + direction + n) % n
			m.config.STTMode = m.sttModes[m.sttModeIndex]
		}
	case idMenuGap:
		m.config.Layout.MarginX = clamp(m.config.Layout.MarginX+direction, menuGapMin, menuGapMax)
		m.config.Layout.MarginY = m.config.Layout.MarginX
		m.config.Layout.Gap = m.config.Layout.MarginX
	case idIconSize:
		m.config.Layout.SquareSize = clamp(m.config.Layout.SquareSize+direction, iconSizeMin, iconSizeMax)
	case idDefaultVoice:
		// Tri-state cycle: off → on → sticky → off.
		// "sticky" implies voice-on initially; the keyboard rewrites
		// DefaultVoice on each Fn toggle so launches resume the last mode.
		switch {
		case !m.defaultVoice && !m.config.DefaultVoiceSticky:
			m.defaultVoice = true
			m.config.DefaultVoice = true
			m.config.DefaultVoiceSticky = false
		case m.defaultVoice && !m.config.DefaultVoiceSticky:
			m.config.DefaultVoiceSticky = true
		default:
			m.defaultVoice = false
			m.config.DefaultVoice = false
			m.config.DefaultVoiceSticky = false
		}
	case idBrightness:
		if m.backlight == nil {
			return
		}
		next := clamp(m.brightnessPct+direction*brightnessStep, brightnessMin, brightnessMax)
		if next == m.brightnessPct {
			return
		}
		if err := m.backlight.writePercent(next); err != nil {
			m.status = err.Error()
			return
		}
		m.brightnessPct = next
		return
	case idRexOnBoot:
		want := !m.rexOnBoot
		if err := setRexOnBoot(want); err != nil {
			m.status = err.Error()
			return
		}
		m.rexOnBoot = want
		if want {
			m.status = "rex enabled on boot"
		} else {
			m.status = "rex disabled on boot"
		}
		return
	case idTimezone:
		if n := len(m.timezones); n > 0 {
			next := (m.tzIndex + direction + n) % n
			tz := m.timezones[next]
			if err := applyTimezone(tz); err != nil {
				m.status = err.Error()
				return
			}
			m.tzIndex = next
			m.config.Timezone = tz
		}
	default:
		return
	}
	if err := cfg.SaveConfig(m.config); err != nil {
		m.status = fmt.Sprintf("save failed: %v", err)
		return
	}
}

func (m *Model) formatRow(name, value string, width int) string {
	if width < 10 {
		width = 10
	}

	if len(name) > width-6 {
		name = name[:width-6]
	}
	if len(value) > width-6 {
		value = value[:width-6]
	}

	spaces := width - len(name) - len(value)
	if spaces < 1 {
		spaces = 1
	}
	return name + strings.Repeat(" ", spaces) + value
}
