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
	KeyboardModeVoice               = "voice"
	DefaultDictWords                = 50000
	configPathEnvVar                = "REX_CONFIG_PATH"
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
	Layout       Layout   `yaml:"layout"`
	Apps         []App    `yaml:"apps"`
	SSHHosts     []string `yaml:"ssh_hosts"`
	KeyboardMode         string `yaml:"keyboard_mode"`
	KeyboardCompact      bool   `yaml:"keyboard_compact,omitempty"`
	KeyboardAutocomplete bool   `yaml:"keyboard_autocomplete,omitempty"`
	DictWords            int    `yaml:"dict_words,omitempty"`
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
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return Config{}, err
	}

	config.KeyboardMode = NormalizeKeyboardMode(config.KeyboardMode)
	return config, nil
}

func saveConfigToPath(path string, config Config) error {
	config.KeyboardMode = NormalizeKeyboardMode(config.KeyboardMode)

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

func NormalizeKeyboardMode(mode string) string {
	switch mode {
	case KeyboardModeSimple, KeyboardModeChorded, KeyboardModeChordedSimultaneous, KeyboardModeFrontChorded, KeyboardModeVoice:
		return mode
	default:
		return KeyboardModeChorded
	}
}

func KeyboardModes() []string {
	return []string{
		KeyboardModeSimple,
		KeyboardModeChorded,
		KeyboardModeChordedSimultaneous,
		KeyboardModeFrontChorded,
		KeyboardModeVoice,
	}
}
