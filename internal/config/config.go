package config

import (
	"os"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/haryoiro/yutemal/internal/structures"
)

// Load loads the configuration from a TOML file.
func Load(path string) (*structures.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := Default()
	if err2 := toml.Unmarshal(data, cfg); err2 != nil {
		return nil, err2
	}

	return cfg, nil
}

// Save saves the configuration to a TOML file.
func Save(cfg *structures.Config, path string) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// Default returns the default configuration.
func Default() *structures.Config {
	return &structures.Config{
		MaxConcurrentDownloads: 4,
		DefaultVolume:          0.7,
		SeekSeconds:            5,
		MaxCacheSize:           1024,   // 1GB
		AudioQuality:           "high", // Default to medium quality
		Theme: structures.Theme{
			Background:       "#1a1b26",  // Tokyo Night Storm background
			Foreground:       "#c0caf5",  // Tokyo Night foreground
			Selected:         "#7aa2f7",  // Tokyo Night blue
			Playing:          "#9ece6a",  // Tokyo Night green
			Border:           "#3b4261",  // Tokyo Night border
			ProgressBar:      "#565f89",  // Tokyo Night dark gray
			ProgressBarFill:  "#7aa2f7",  // Tokyo Night blue
			ProgressBarStyle: "gradient", // Default to gradient style
		},
		KeyBindings: structures.KeyBindings{
			// Global controls
			PlayPause:    "space",
			Quit:         "ctrl+c",
			VolumeUp:     []string{"+", "="},
			VolumeDown:   []string{"-", "_"},
			SeekForward:  "right",
			SeekBackward: "left",

			// Navigation
			MoveUp:      []string{"up", "k"},
			MoveDown:    []string{"down", "j"},
			Select:      []string{"enter", "l"},
			Back:        []string{"backspace", "b"}, // ESC removed to prevent mouse-triggered navigation
			NextSection: "tab",
			PrevSection: "shift+tab",

			// Actions
			Search:      "f",
			Shuffle:     "s",
			RemoveTrack: "r",
			Home:        "h",
		},
	}
}
