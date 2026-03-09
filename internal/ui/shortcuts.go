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
		if after, ok := strings.CutPrefix(key, "ctrl+"); ok {
			formatted = "Ctrl+" + strings.ToUpper(after)
		} else if strings.HasPrefix(key, "cmd+") || strings.HasPrefix(key, "meta+") {
			formatted = "Cmd+" + strings.ToUpper(strings.TrimPrefix(strings.TrimPrefix(key, "cmd+"), "meta+"))
		} else if strings.HasPrefix(key, "alt+") || strings.HasPrefix(key, "opt+") {
			formatted = "Alt+" + strings.ToUpper(strings.TrimPrefix(strings.TrimPrefix(key, "alt+"), "opt+"))
		} else if len(key) == 1 {
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

	sortedKeys := make([]string, len(keys))
	copy(sortedKeys, keys)

	for i := range sortedKeys {
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
	isArrow1 := isArrowKey(key1)
	isArrow2 := isArrowKey(key2)

	if isArrow1 && !isArrow2 {
		return false
	}

	if !isArrow1 && isArrow2 {
		return true
	}

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

// GetPlayerHints returns shortcuts for the player panel.
func (sf *ShortcutFormatter) GetPlayerHints(focused bool) []ShortcutHint {
	kb := sf.config.KeyBindings

	if focused {
		return []ShortcutHint{
			{Key: sf.formatKey(kb.PlayPause), Action: "Play/Pause"},
			{Key: upArrow + "/" + downArrow, Action: "Volume"},
			{Key: leftArrow + "/" + rightArrow, Action: "Seek"},
			{Key: sf.formatKey(kb.ToggleEQ), Action: "EQ"},
			{Key: sf.formatKey("tab"), Action: "Next Pane"},
		}
	}

	return []ShortcutHint{
		{Key: sf.formatKey(kb.PlayPause), Action: "Play/Pause"},
		{Key: sf.formatKey(kb.SeekBackward) + "/" + sf.formatKey(kb.SeekForward), Action: "Seek"},
		{Key: sf.formatKey(kb.ToggleEQ), Action: "EQ"},
		{Key: sf.formatKey(kb.Quit), Action: "Quit"},
	}
}

// GetPlaylistListHints returns shortcuts for the playlist list view.
func (sf *ShortcutFormatter) GetPlaylistListHints(showQueue bool) []ShortcutHint {
	kb := sf.config.KeyBindings
	hints := []ShortcutHint{
		{Key: sf.formatKeys(kb.Select), Action: "Open"},
		{Key: sf.formatKeys(kb.Search), Action: "Search"},
		{Key: sf.formatKey("tab"), Action: "Next Pane"},
	}

	if showQueue {
		hints = append(hints, ShortcutHint{Key: "q", Action: "Hide Queue"})
	} else {
		hints = append(hints, ShortcutHint{Key: "q", Action: "Show Queue"})
	}

	return hints
}

// GetPlaylistHints returns shortcuts for playlist detail views.
func (sf *ShortcutFormatter) GetPlaylistHints(showQueue bool) []ShortcutHint {
	kb := sf.config.KeyBindings
	hints := []ShortcutHint{
		{Key: sf.formatKeys(kb.Select), Action: "Play from Here"},
		{Key: "a", Action: "Add Next"},
		{Key: sf.formatKey(kb.RemoveTrack), Action: "Remove"},
		{Key: sf.formatKeys(kb.Back), Action: "Back"},
		{Key: sf.formatKey("tab"), Action: "Next Pane"},
	}

	if showQueue {
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

// GetQueueHints returns queue-specific shortcuts.
func (sf *ShortcutFormatter) GetQueueHints(hasFocus bool) []ShortcutHint {
	kb := sf.config.KeyBindings
	hints := []ShortcutHint{
		{Key: sf.formatKey("tab"), Action: "Next Pane"},
		{Key: sf.formatKeys(kb.MoveUp) + "/" + sf.formatKeys(kb.MoveDown), Action: "Navigate"},
		{Key: sf.formatKeys(kb.Select), Action: "Play"},
		{Key: sf.formatKey(kb.RemoveTrack), Action: "Remove"},
	}

	if !hasFocus {
		hints = []ShortcutHint{
			{Key: sf.formatKey("tab"), Action: "Focus Queue"},
		}
	}

	return hints
}

// GetSearchHints returns search view shortcuts.
func (sf *ShortcutFormatter) GetSearchHints() []ShortcutHint {
	kb := sf.config.KeyBindings

	return []ShortcutHint{
		{Key: sf.formatKey("enter"), Action: "Search"},
		{Key: sf.formatKeys(kb.Back), Action: "Cancel"},
	}
}

// GetContextualHints returns shortcuts based on the current UI state.
func (sf *ShortcutFormatter) GetContextualHints(state ViewState, showQueue bool, hasFocus func(string) bool) string {
	if hasFocus("player") {
		return sf.FormatHints(sf.GetPlayerHints(true))
	}

	if hasFocus("queue") {
		return sf.FormatHints(sf.GetQueueHints(true))
	}

	switch state {
	case SearchView:
		return sf.FormatHints(sf.GetSearchHints())
	default:
		return ""
	}
}

// GetEmptyStateHint returns a hint for empty states.
func (sf *ShortcutFormatter) GetEmptyStateHint(action string, key string) string {
	formattedKey := sf.formatKey(key)
	return fmt.Sprintf("Press '%s' to %s", formattedKey, action)
}
