package config

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	KeyboardModeSimple              = "simple"
	KeyboardModeChorded             = "chorded"
	KeyboardModeChordedSimultaneous = "chorded-simultaneous"
	KeyboardModeFrontChorded        = "front-chorded"
	KeyboardViewGrid                = "grid"
	KeyboardViewFan                 = "fan"
	KeyboardLayout4x8               = "4x8"
	KeyboardLayout3Row              = "3-row"
	DefaultDictWords                = 50000
	configPathEnvVar                = "REX_CONFIG_PATH"

	STTModeSherpa    = "sherpa"    // on-device STT via sherpa-onnx, NeMo Conformer-Small (Brick default)
	STTModeParakeet  = "parakeet"  // on-device STT via sherpa-onnx, Parakeet-TDT-CTC 110M (Brick, with punctuation + casing)
	STTModeCompanion = "companion" // iOS companion app STT (R36S)
)

type App struct {
	Name      string `yaml:"name"`
	Icon      string `yaml:"icon"`
	Color     string `yaml:"color"`
	Command   string `yaml:"command"`
	Exclusive bool   `yaml:"exclusive"`
}

type Layout struct {
	Gap        int `yaml:"gap"`
	MarginX    int `yaml:"margin_x"`
	MarginY    int `yaml:"margin_y"`
	SquareSize int `yaml:"square_size"`
}

type Config struct {
	Layout               Layout   `yaml:"layout"`
	Apps                 []App    `yaml:"apps"`
	SSHHosts             []string `yaml:"ssh_hosts"`
	KeyboardMode         string   `yaml:"keyboard_mode"`
	KeyboardLayout       string   `yaml:"keyboard_layout,omitempty"`
	KeyboardView         string   `yaml:"keyboard_view,omitempty"`
	KeyboardCompact      bool     `yaml:"keyboard_compact"`
	KeyboardAutocomplete bool     `yaml:"keyboard_autocomplete,omitempty"`
	DictWords            int      `yaml:"dict_words,omitempty"`
	STTMode              string   `yaml:"stt_mode,omitempty"`      // "sherpa" (default), "parakeet", or "companion"
	DefaultVoice         bool     `yaml:"default_voice,omitempty"` // open keyboard in voice mode by default
}

func LoadConfig() (Config, error) {
	configPath, err := resolveConfigPath()
	if err != nil {
		return Config{}, err
	}
	return loadConfigFromPath(configPath)
}

func SaveConfig(config Config) error {
	configPath, err := resolveConfigPath()
	if err != nil {
		return err
	}
	return saveConfigToPath(configPath, config)
}

func loadConfigFromPath(path string) (Config, error) {
	config := defaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return config, nil
		}
		return Config{}, err
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return Config{}, err
	}

	config.KeyboardMode = NormalizeKeyboardMode(config.KeyboardMode)
	config.KeyboardLayout = NormalizeKeyboardLayout(config.KeyboardLayout)
	config.KeyboardView = NormalizeKeyboardView(config.KeyboardView)
	return config, nil
}

func defaultConfig() Config {
	return Config{
		KeyboardMode:    KeyboardModeChorded,
		KeyboardLayout:  KeyboardLayout4x8,
		KeyboardView:    KeyboardViewGrid,
		KeyboardCompact: true,
	}
}

func saveConfigToPath(path string, config Config) error {
	config.KeyboardMode = NormalizeKeyboardMode(config.KeyboardMode)
	config.KeyboardLayout = NormalizeKeyboardLayout(config.KeyboardLayout)
	config.KeyboardView = NormalizeKeyboardView(config.KeyboardView)
	config.STTMode = NormalizeSTTMode(config.STTMode)

	data, err := yaml.Marshal(&config)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// resolveConfigPath returns the config file path to use. It checks (in order):
// the REX_CONFIG_PATH env var, ~/.config/rex/config.yaml, and ./config.yaml.
// If none exist, it returns the default ~/.config/rex/config.yaml path — the
// file may not exist yet, and callers handle that case.
func resolveConfigPath() (string, error) {
	if envPath := os.Getenv(configPathEnvVar); envPath != "" {
		return envPath, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	defaultPath := filepath.Join(homeDir, ".config", "rex", "config.yaml")

	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath, nil
	}

	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml", nil
	}

	return defaultPath, nil
}

func NormalizeSTTMode(mode string) string {
	switch mode {
	case STTModeSherpa, STTModeParakeet, STTModeCompanion:
		return mode
	default:
		return STTModeSherpa
	}
}

func NormalizeKeyboardMode(mode string) string {
	switch mode {
	case KeyboardModeSimple, KeyboardModeChorded, KeyboardModeChordedSimultaneous, KeyboardModeFrontChorded:
		return mode
	default:
		return KeyboardModeChorded
	}
}

func NormalizeKeyboardView(view string) string {
	switch view {
	case KeyboardViewGrid, KeyboardViewFan:
		return view
	default:
		return KeyboardViewGrid
	}
}

func NormalizeKeyboardLayout(layout string) string {
	switch layout {
	case KeyboardLayout4x8, KeyboardLayout3Row:
		return layout
	default:
		return KeyboardLayout4x8
	}
}

// KeyboardPaneHeight returns the tmux pane height for the default keyboard layout.
func KeyboardPaneHeight(compact, autocomplete bool) int {
	return KeyboardPaneHeightForLayout(compact, autocomplete, KeyboardLayout4x8)
}

// KeyboardPaneHeightForLayout returns the tmux pane height for the keyboard pane.
// Compact mode stacks rows of 2-line keys; normal uses 3-line keys.
// Autocomplete adds a one-line suggestion bar above the keys.
func KeyboardPaneHeightForLayout(compact, autocomplete bool, layout string) int {
	rowCount := 4
	if NormalizeKeyboardLayout(layout) == KeyboardLayout3Row {
		rowCount = 3
	}

	rowHeight := 3
	if compact {
		rowHeight = 2
	}

	rows := rowCount * rowHeight
	if autocomplete {
		rows++
	}
	return rows
}

func STTModes() []string {
	return []string{
		STTModeSherpa,
		STTModeParakeet,
		STTModeCompanion,
	}
}

func KeyboardModes() []string {
	return []string{
		KeyboardModeSimple,
		KeyboardModeChorded,
		KeyboardModeChordedSimultaneous,
		KeyboardModeFrontChorded,
	}
}

func KeyboardViews() []string {
	return []string{
		KeyboardViewGrid,
		KeyboardViewFan,
	}
}

func KeyboardLayouts() []string {
	return []string{
		KeyboardLayout4x8,
		KeyboardLayout3Row,
	}
}
