package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	// Editor is the command to use for external editing (defaults to $EDITOR or nvim)
	Editor string `yaml:"editor"`
	// EditorArgs are additional arguments to pass to the editor
	EditorArgs []string `yaml:"editor_args"`
	// Theme settings
	Theme ThemeConfig `yaml:"theme"`
	// Keybinds settings
	Keybinds KeybindConfig `yaml:"keybinds"`
}

// KeybindConfig holds keybind-related settings
type KeybindConfig struct {
	// LeaderKey is the global leader key for navigation (default: "ctrl+space")
	LeaderKey string `yaml:"leader_key"`
	// Navigation keybinds (used after leader key)
	Navigation NavigationKeybinds `yaml:"navigation"`
	// Global keybinds (without leader)
	Global GlobalKeybinds `yaml:"global"`
}

// NavigationKeybinds holds panel navigation keybinds (used after leader key)
type NavigationKeybinds struct {
	Conversations string `yaml:"conversations"` // default: "c"
	Messages      string `yaml:"messages"`      // default: "m"
	Input         string `yaml:"input"`         // default: "i"
}

// GlobalKeybinds holds global keybinds (without leader)
type GlobalKeybinds struct {
	Quit       string `yaml:"quit"`        // default: "q"
	NextPanel  string `yaml:"next_panel"`  // default: "tab"
	PrevPanel  string `yaml:"prev_panel"`  // default: "shift+tab"
	Help       string `yaml:"help"`        // default: "?"
	Refresh    string `yaml:"refresh"`     // default: "ctrl+r"
}

// ThemeConfig holds theme-related settings
type ThemeConfig struct {
	// PrimaryColor is the main accent color (hex)
	PrimaryColor string `yaml:"primary_color"`
	// SecondaryColor is the secondary accent color (hex)
	SecondaryColor string `yaml:"secondary_color"`
}

// DefaultConfig returns a Config with default values
func DefaultConfig() *Config {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nvim"
	}

	return &Config{
		Editor:     editor,
		EditorArgs: []string{},
		Theme: ThemeConfig{
			PrimaryColor:   "#7C3AED",
			SecondaryColor: "#10B981",
		},
		Keybinds: DefaultKeybinds(),
	}
}

// DefaultKeybinds returns the default keybind configuration
func DefaultKeybinds() KeybindConfig {
	return KeybindConfig{
		LeaderKey: "ctrl+space",
		Navigation: NavigationKeybinds{
			Conversations: "c",
			Messages:      "m",
			Input:         "i",
		},
		Global: GlobalKeybinds{
			Quit:      "q",
			NextPanel: "tab",
			PrevPanel: "shift+tab",
			Help:      "?",
			Refresh:   "ctrl+r",
		},
	}
}

// ConfigDir returns the path to the config directory
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "messages-tui"), nil
}

// ConfigPath returns the path to the config file
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// Load loads the configuration from disk, or returns defaults if not found
func Load() (*Config, error) {
	cfg := DefaultConfig()

	path, err := ConfigPath()
	if err != nil {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Ensure editor is set even if config file has empty value
	if cfg.Editor == "" {
		cfg.Editor = DefaultConfig().Editor
	}

	// Merge keybind defaults for any unset values
	defaults := DefaultKeybinds()
	if cfg.Keybinds.LeaderKey == "" {
		cfg.Keybinds.LeaderKey = defaults.LeaderKey
	}
	if cfg.Keybinds.Navigation.Conversations == "" {
		cfg.Keybinds.Navigation.Conversations = defaults.Navigation.Conversations
	}
	if cfg.Keybinds.Navigation.Messages == "" {
		cfg.Keybinds.Navigation.Messages = defaults.Navigation.Messages
	}
	if cfg.Keybinds.Navigation.Input == "" {
		cfg.Keybinds.Navigation.Input = defaults.Navigation.Input
	}
	if cfg.Keybinds.Global.Quit == "" {
		cfg.Keybinds.Global.Quit = defaults.Global.Quit
	}
	if cfg.Keybinds.Global.NextPanel == "" {
		cfg.Keybinds.Global.NextPanel = defaults.Global.NextPanel
	}
	if cfg.Keybinds.Global.PrevPanel == "" {
		cfg.Keybinds.Global.PrevPanel = defaults.Global.PrevPanel
	}
	if cfg.Keybinds.Global.Help == "" {
		cfg.Keybinds.Global.Help = defaults.Global.Help
	}
	if cfg.Keybinds.Global.Refresh == "" {
		cfg.Keybinds.Global.Refresh = defaults.Global.Refresh
	}

	return cfg, nil
}

// Save saves the configuration to disk
func (c *Config) Save() error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path, err := ConfigPath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// EnsureConfigDir creates the config directory if it doesn't exist
func EnsureConfigDir() error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(dir, 0755)
}
