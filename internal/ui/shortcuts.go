package ui

import (
	"fmt"
	"strings"

	"github.com/haryoiro/yutemal/internal/structures"
)

// ShortcutHint represents a single keyboard shortcut hint
type ShortcutHint struct {
	Key    string
	Action string
}

// ShortcutGroup represents a group of related shortcuts
type ShortcutGroup struct {
	Hints []ShortcutHint
}

// ShortcutFormatter handles formatting of keyboard shortcuts for display
type ShortcutFormatter struct {
	config     *structures.Config
	styleCache map[string]string
}

// NewShortcutFormatter creates a new shortcut formatter with the given config
func NewShortcutFormatter(config *structures.Config) *ShortcutFormatter {
	return &ShortcutFormatter{
		config:     config,
		styleCache: make(map[string]string),
	}
}

// formatKey formats a key binding for display
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
		formatted = "↑"
	case "down":
		formatted = "↓"
	case "left":
		formatted = "←"
	case "right":
		formatted = "→"
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

// formatKeys formats multiple key bindings (e.g., ["down", "j"] -> "↓/j")
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

// shouldSwapKeys returns true if key1 should come after key2
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

// isArrowKey checks if a key is an arrow key
func isArrowKey(key string) bool {
	return key == "up" || key == "down" || key == "left" || key == "right"
}

// FormatHint formats a single shortcut hint
func (sf *ShortcutFormatter) FormatHint(hint ShortcutHint) string {
	return fmt.Sprintf("[%s: %s]", hint.Key, hint.Action)
}

// FormatHints formats multiple shortcut hints with consistent styling
func (sf *ShortcutFormatter) FormatHints(hints []ShortcutHint) string {
	formatted := make([]string, len(hints))
	for i, hint := range hints {
		formatted[i] = sf.FormatHint(hint)
	}
	return strings.Join(formatted, " ")
}

// GetPlayerHints returns shortcuts for the player view
func (sf *ShortcutFormatter) GetPlayerHints() []ShortcutHint {
	kb := sf.config.KeyBindings
	return []ShortcutHint{
		{Key: sf.formatKey(kb.PlayPause), Action: "Play/Pause"},
		{Key: sf.formatKey(kb.SeekBackward) + "/" + sf.formatKey(kb.SeekForward), Action: "Seek"},
		{Key: sf.formatKey(kb.Shuffle), Action: "Shuffle"},
		{Key: "q", Action: "Show Queue"},
		{Key: sf.formatKey(kb.Quit), Action: "Quit"},
	}
}

// GetPlaylistHints returns shortcuts for playlist views
func (sf *ShortcutFormatter) GetPlaylistHints() []ShortcutHint {
	kb := sf.config.KeyBindings
	return []ShortcutHint{
		{Key: sf.formatKeys(kb.Select), Action: "Play from Here"},
		{Key: sf.formatKey(kb.RemoveTrack), Action: "Remove Track"},
		{Key: "q", Action: "Show Queue"},
	}
}

// GetNavigationHints returns navigation shortcuts
func (sf *ShortcutFormatter) GetNavigationHints() []ShortcutHint {
	kb := sf.config.KeyBindings
	return []ShortcutHint{
		{Key: sf.formatKeys(kb.MoveUp) + "/" + sf.formatKeys(kb.MoveDown), Action: "Navigate"},
		{Key: sf.formatKeys(kb.Select), Action: "Select"},
	}
}

// GetQueueHints returns queue-specific shortcuts
func (sf *ShortcutFormatter) GetQueueHints(hasFocus bool) []ShortcutHint {
	kb := sf.config.KeyBindings
	hints := []ShortcutHint{
		{Key: sf.formatKeys(kb.MoveUp) + "/" + sf.formatKeys(kb.MoveDown), Action: "Navigate"},
		{Key: sf.formatKeys(kb.Select), Action: "Play from Here"},
		{Key: sf.formatKey(kb.RemoveTrack), Action: "Remove Track"},
	}

	// Add focus-specific hint
	if hasFocus {
		hints = append([]ShortcutHint{{Key: sf.formatKey("tab"), Action: "Return to List"}}, hints...)
	} else {
		hints = append([]ShortcutHint{{Key: sf.formatKey("tab"), Action: "Switch to Queue"}}, hints...)
	}

	return hints
}

// GetSearchHints returns search view shortcuts
func (sf *ShortcutFormatter) GetSearchHints() []ShortcutHint {
	kb := sf.config.KeyBindings
	return []ShortcutHint{
		{Key: sf.formatKeys(kb.Select), Action: "Search"},
		{Key: sf.formatKey("esc"), Action: "Cancel"},
	}
}

// GetContextualHints returns shortcuts based on the current UI state
func (sf *ShortcutFormatter) GetContextualHints(state ViewState, showQueue bool, hasFocus func(string) bool) string {
	kb := sf.config.KeyBindings

	switch state {
	case HomeView:
		if showQueue {
			return sf.FormatHints([]ShortcutHint{
				{Key: sf.formatKey("tab"), Action: "Switch to Queue"},
				{Key: sf.formatKey(kb.Search), Action: "Search"},
			})
		}
		return sf.FormatHints([]ShortcutHint{
			{Key: "q", Action: "Show Queue"},
			{Key: sf.formatKey(kb.Search), Action: "search"},
		})

	case PlaylistDetailView:
		if showQueue {
			return sf.FormatHints([]ShortcutHint{
				{Key: sf.formatKey("tab"), Action: "Switch to Queue"},
				{Key: sf.formatKeys(kb.Back), Action: "Go Home"},
			})
		}
		return sf.FormatHints([]ShortcutHint{
			{Key: "q", Action: "Show Queue"},
			{Key: sf.formatKeys(kb.Back), Action: "Go Home"},
		})

	case SearchView:
		return sf.FormatHints(sf.GetSearchHints())

	default:
		if hasFocus("queue") {
			return sf.FormatHints([]ShortcutHint{
				{Key: sf.formatKey("tab"), Action: "Return to List"},
				{Key: "q", Action: "Hide Queue"},
			})
		}
		return ""
	}
}

// GetEmptyStateHint returns a hint for empty states
func (sf *ShortcutFormatter) GetEmptyStateHint(action string, key string) string {
	formattedKey := sf.formatKey(key)
	return fmt.Sprintf("Press '%s' to %s", formattedKey, action)
}

// GetSectionNavigationHint returns section navigation hints
func (sf *ShortcutFormatter) GetSectionNavigationHint(canSwitch bool) string {
	if !canSwitch {
		return fmt.Sprintf("%s to switch sections disabled (queue is shown)", sf.formatKey("tab"))
	}
	return fmt.Sprintf("(%s/%s to switch)", sf.formatKey("tab"), sf.formatKey("shift+tab"))
}