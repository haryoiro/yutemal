package ui

import (
	"fmt"
	"strings"

	"github.com/haryoiro/yutemal/internal/structures"
)

const (
	upArrow    = "↑"
	downArrow  = "↓"
	leftArrow  = "←"
	rightArrow = "→"
)

// ShortcutHint represents a single keyboard shortcut hint.
type ShortcutHint struct {
	Key    string
	Action string
}

// ShortcutGroup represents a group of related shortcuts.
type ShortcutGroup struct {
	Hints []ShortcutHint
}

// ShortcutFormatter handles formatting of keyboard shortcuts for display.
type ShortcutFormatter struct {
	config     *structures.Config
	styleCache map[string]string
}

// NewShortcutFormatter creates a new shortcut formatter with the given config.
func NewShortcutFormatter(config *structures.Config) *ShortcutFormatter {
	return &ShortcutFormatter{
		config:     config,
		styleCache: make(map[string]string),
	}
}

// formatKey formats a key binding for display.
func (sf *ShortcutFormatter) formatKey(key string) string {
	// Use cache to avoid repeated formatting
	if formatted, ok := sf.styleCache[key]; ok {
		return formatted
	}

	formatted := key

	switch key {
	case "space":
		formatted = "Space"
	case "enter":
		formatted = "Enter"
	case "esc":
		formatted = "Esc"
	case "tab":
		formatted = "Tab"
	case "shift+tab":
		formatted = "Shift+Tab"
	case "backspace":
		formatted = "Back"
	case "ctrl+c":
		formatted = "Ctrl+C"
	case "ctrl+d":
		formatted = "Ctrl+D"
	case "cmd+c", "meta+c":
		formatted = "Cmd+C"
	case "alt+c", "opt+c":
		formatted = "Alt+C"
	case "up":
		formatted = upArrow
	case "down":
		formatted = downArrow
	case "left":
		formatted = leftArrow
	case "right":
		formatted = rightArrow
	case "pgup":
		formatted = "PgUp"
	case "pgdown":
		formatted = "PgDn"
	default:
		// Handle other ctrl/cmd/alt combinations
		if strings.HasPrefix(key, "ctrl+") {
			formatted = "Ctrl+" + strings.ToUpper(strings.TrimPrefix(key, "ctrl+"))
		} else if strings.HasPrefix(key, "cmd+") || strings.HasPrefix(key, "meta+") {
			formatted = "Cmd+" + strings.ToUpper(strings.TrimPrefix(strings.TrimPrefix(key, "cmd+"), "meta+"))
		} else if strings.HasPrefix(key, "alt+") || strings.HasPrefix(key, "opt+") {
			formatted = "Alt+" + strings.ToUpper(strings.TrimPrefix(strings.TrimPrefix(key, "alt+"), "opt+"))
		} else if len(key) == 1 {
			// Single character keys stay as-is
			formatted = key
		}
	}

	sf.styleCache[key] = formatted

	return formatted
}

// formatKeys formats multiple key bindings (e.g., ["down", "j"] -> "↓/j").
func (sf *ShortcutFormatter) formatKeys(keys []string) string {
	if len(keys) == 0 {
		return ""
	}

	// Sort keys to ensure arrow keys come first
	sortedKeys := make([]string, len(keys))
	copy(sortedKeys, keys)

	// Custom sort: arrow keys first, then alphabetical
	for i := 0; i < len(sortedKeys); i++ {
		for j := i + 1; j < len(sortedKeys); j++ {
			if shouldSwapKeys(sortedKeys[i], sortedKeys[j]) {
				sortedKeys[i], sortedKeys[j] = sortedKeys[j], sortedKeys[i]
			}
		}
	}

	formatted := make([]string, len(sortedKeys))
	for i, key := range sortedKeys {
		formatted[i] = sf.formatKey(key)
	}

	return strings.Join(formatted, "/")
}

// shouldSwapKeys returns true if key1 should come after key2.
func shouldSwapKeys(key1, key2 string) bool {
	// Arrow keys have priority
	isArrow1 := isArrowKey(key1)
	isArrow2 := isArrowKey(key2)

	if isArrow1 && !isArrow2 {
		return false // key1 (arrow) should come first
	}

	if !isArrow1 && isArrow2 {
		return true // key2 (arrow) should come first
	}

	// If both are arrows or both are not arrows, use alphabetical order
	return key1 > key2
}

// isArrowKey checks if a key is an arrow key.
func isArrowKey(key string) bool {
	return key == "up" || key == "down" || key == "left" || key == "right"
}

// FormatHint formats a single shortcut hint.
func (sf *ShortcutFormatter) FormatHint(hint ShortcutHint) string {
	return fmt.Sprintf("[%s: %s]", hint.Key, hint.Action)
}

// FormatHints formats multiple shortcut hints with consistent styling.
func (sf *ShortcutFormatter) FormatHints(hints []ShortcutHint) string {
	formatted := make([]string, len(hints))
	for i, hint := range hints {
		formatted[i] = sf.FormatHint(hint)
	}

	return strings.Join(formatted, " ")
}

// GetPlayerHints returns shortcuts for the player view.
func (sf *ShortcutFormatter) GetPlayerHints(isHomeView bool, hasMultipleSections bool) []ShortcutHint {
	kb := sf.config.KeyBindings
	hints := []ShortcutHint{
		{Key: sf.formatKey(kb.PlayPause), Action: "Play/Pause"},
	}

	// Show seek hint only when not in home view with multiple sections
	if !isHomeView || !hasMultipleSections {
		hints = append(hints, ShortcutHint{
			Key:    sf.formatKey(kb.SeekBackward) + "/" + sf.formatKey(kb.SeekForward),
			Action: "Seek",
		})
	}

	hints = append(hints,
		ShortcutHint{Key: sf.formatKey(kb.Shuffle), Action: "Shuffle"},
		ShortcutHint{Key: sf.formatKey(kb.Quit), Action: "Quit"},
	)

	return hints
}

// GetPlaylistHints returns shortcuts for playlist views.
func (sf *ShortcutFormatter) GetPlaylistHints(showQueue bool) []ShortcutHint {
	kb := sf.config.KeyBindings
	hints := []ShortcutHint{
		{Key: sf.formatKeys(kb.Select), Action: "Play from Here"},
		{Key: "a", Action: "Add Next"},
		{Key: sf.formatKey(kb.RemoveTrack), Action: "Remove Track"},
		{Key: sf.formatKeys(kb.Back), Action: "Back"},
	}

	if showQueue {
		hints = append(hints, ShortcutHint{Key: sf.formatKey("tab"), Action: "Focus Queue"})
		hints = append(hints, ShortcutHint{Key: "q", Action: "Hide Queue"})
	} else {
		hints = append(hints, ShortcutHint{Key: "q", Action: "Show Queue"})
	}

	return hints
}

// GetNavigationHints returns navigation shortcuts.
func (sf *ShortcutFormatter) GetNavigationHints() []ShortcutHint {
	kb := sf.config.KeyBindings

	return []ShortcutHint{
		{Key: sf.formatKeys(kb.MoveUp) + "/" + sf.formatKeys(kb.MoveDown), Action: "Navigate"},
		{Key: sf.formatKeys(kb.Select), Action: "Select"},
	}
}

// GetHomeHints returns shortcuts for the home view.
func (sf *ShortcutFormatter) GetHomeHints(showQueue bool, hasMultipleSections bool) []ShortcutHint {
	kb := sf.config.KeyBindings
	hints := []ShortcutHint{
		{Key: sf.formatKeys(kb.Select), Action: "Open"},
	}

	// Show section navigation only when queue is not shown and there are multiple sections
	if !showQueue && hasMultipleSections {
		hints = append(hints, ShortcutHint{Key: "←/→", Action: "Switch Section"})
	}

	hints = append(hints, ShortcutHint{Key: sf.formatKey(kb.Search), Action: "Search"})

	if showQueue {
		hints = append(hints, ShortcutHint{Key: sf.formatKey("tab"), Action: "Focus Queue"})
		hints = append(hints, ShortcutHint{Key: "q", Action: "Hide Queue"})
	} else {
		hints = append(hints, ShortcutHint{Key: "q", Action: "Show Queue"})
	}

	return hints
}

// GetQueueHints returns queue-specific shortcuts.
func (sf *ShortcutFormatter) GetQueueHints(hasFocus bool) []ShortcutHint {
	kb := sf.config.KeyBindings
	hints := []ShortcutHint{
		{Key: sf.formatKeys(kb.MoveUp) + "/" + sf.formatKeys(kb.MoveDown), Action: "Navigate"},
		{Key: sf.formatKeys(kb.Select), Action: "Play from Here"},
		{Key: sf.formatKey(kb.RemoveTrack), Action: "Remove Track"},
	}

	// Add focus-specific hint
	if hasFocus {
		hints = append([]ShortcutHint{{Key: sf.formatKey("tab"), Action: "Change Focus"}}, hints...)
	} else {
		hints = append([]ShortcutHint{{Key: sf.formatKey("tab"), Action: "Focus Queue"}}, hints...)
	}

	return hints
}

// GetSearchHints returns search view shortcuts.
func (sf *ShortcutFormatter) GetSearchHints() []ShortcutHint {
	kb := sf.config.KeyBindings

	return []ShortcutHint{
		{Key: sf.formatKeys(kb.Select), Action: "Search"},
		{Key: sf.formatKey("esc"), Action: "Cancel"},
	}
}

// GetContextualHints returns shortcuts based on the current UI state.
func (sf *ShortcutFormatter) GetContextualHints(state ViewState, showQueue bool, hasFocus func(string) bool) string {
	switch state {
	case HomeView:
		// Home view shortcuts are shown in the header
		return ""

	case PlaylistDetailView:
		// Playlist view shortcuts are shown in the header
		return ""

	case SearchView:
		return sf.FormatHints(sf.GetSearchHints())

	default:
		if hasFocus("queue") {
			return sf.FormatHints([]ShortcutHint{
				{Key: sf.formatKey("tab"), Action: "Change Focus"},
				{Key: "q", Action: "Hide Queue"},
			})
		}

		return ""
	}
}

// GetEmptyStateHint returns a hint for empty states.
func (sf *ShortcutFormatter) GetEmptyStateHint(action string, key string) string {
	formattedKey := sf.formatKey(key)
	return fmt.Sprintf("Press '%s' to %s", formattedKey, action)
}

// GetSectionNavigationHint returns section navigation hints.
func (sf *ShortcutFormatter) GetSectionNavigationHint(hasMultipleSections bool) string {
	// Section navigation is not available due to key conflicts
	// Tab is used for queue focus, Left/Right for seeking
	return ""
}
