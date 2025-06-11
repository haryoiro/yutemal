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
		formatted = "␣" // or "Space" if preferred
	case "enter":
		formatted = "⏎" // or "Enter" if preferred
	case "esc":
		formatted = "⎋" // or "Esc" if preferred
	case "tab":
		formatted = "⇥" // or "Tab" if preferred
	case "shift+tab":
		formatted = "⇤" // or "Shift+Tab" if preferred
	case "backspace":
		formatted = "⌫" // or "Back" if preferred
	case "ctrl+c":
		formatted = "^C" // or "Ctrl+C" if preferred
	case "ctrl+d":
		formatted = "^D" // or "Ctrl+D" if preferred
	case "cmd+c", "meta+c":
		formatted = "⌘C"
	case "alt+c", "opt+c":
		formatted = "⌥C"
	case "up":
		formatted = "↑"
	case "down":
		formatted = "↓"
	case "left":
		formatted = "←"
	case "right":
		formatted = "→"
	case "pgup":
		formatted = "⇞" // or "PgUp" if preferred
	case "pgdown":
		formatted = "⇟" // or "PgDn" if preferred
	default:
		// Handle other ctrl/cmd/alt combinations
		if strings.HasPrefix(key, "ctrl+") {
			formatted = "^" + strings.TrimPrefix(key, "ctrl+")
		} else if strings.HasPrefix(key, "cmd+") || strings.HasPrefix(key, "meta+") {
			formatted = "⌘" + strings.TrimPrefix(strings.TrimPrefix(key, "cmd+"), "meta+")
		} else if strings.HasPrefix(key, "alt+") || strings.HasPrefix(key, "opt+") {
			formatted = "⌥" + strings.TrimPrefix(strings.TrimPrefix(key, "alt+"), "opt+")
		} else if len(key) == 1 {
			// Single character keys stay as-is
			formatted = key
		}
	}

	sf.styleCache[key] = formatted
	return formatted
}

// formatKeys formats multiple key bindings (e.g., ["j", "down"] -> "j/↓")
func (sf *ShortcutFormatter) formatKeys(keys []string) string {
	if len(keys) == 0 {
		return ""
	}

	formatted := make([]string, len(keys))
	for i, key := range keys {
		formatted[i] = sf.formatKey(key)
	}
	return strings.Join(formatted, "/")
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
		{Key: "q", Action: "Queue"},
		{Key: sf.formatKey(kb.Quit), Action: "Quit"},
	}
}

// GetPlaylistHints returns shortcuts for playlist views
func (sf *ShortcutFormatter) GetPlaylistHints() []ShortcutHint {
	kb := sf.config.KeyBindings
	return []ShortcutHint{
		{Key: sf.formatKeys(kb.Select), Action: "play"},
		{Key: sf.formatKey(kb.RemoveTrack), Action: "delete"},
		{Key: "q", Action: "queue"},
	}
}

// GetNavigationHints returns navigation shortcuts
func (sf *ShortcutFormatter) GetNavigationHints() []ShortcutHint {
	kb := sf.config.KeyBindings
	return []ShortcutHint{
		{Key: sf.formatKeys(kb.MoveUp) + "/" + sf.formatKeys(kb.MoveDown), Action: "nav"},
		{Key: sf.formatKeys(kb.Select), Action: "select"},
	}
}

// GetQueueHints returns queue-specific shortcuts
func (sf *ShortcutFormatter) GetQueueHints(hasFocus bool) []ShortcutHint {
	kb := sf.config.KeyBindings
	hints := []ShortcutHint{
		{Key: sf.formatKeys(kb.MoveUp) + "/" + sf.formatKeys(kb.MoveDown), Action: "nav"},
		{Key: sf.formatKeys(kb.Select), Action: "play"},
		{Key: sf.formatKey(kb.RemoveTrack), Action: "delete"},
	}

	// Add focus-specific hint
	if hasFocus {
		hints = append([]ShortcutHint{{Key: sf.formatKey("tab"), Action: "back to main"}}, hints...)
	} else {
		hints = append([]ShortcutHint{{Key: sf.formatKey("tab"), Action: "focus queue"}}, hints...)
	}

	return hints
}

// GetSearchHints returns search view shortcuts
func (sf *ShortcutFormatter) GetSearchHints() []ShortcutHint {
	kb := sf.config.KeyBindings
	return []ShortcutHint{
		{Key: sf.formatKeys(kb.Select), Action: "search"},
		{Key: sf.formatKey("esc"), Action: "cancel"},
	}
}

// GetContextualHints returns shortcuts based on the current UI state
func (sf *ShortcutFormatter) GetContextualHints(state ViewState, showQueue bool, hasFocus func(string) bool) string {
	kb := sf.config.KeyBindings

	switch state {
	case HomeView:
		if showQueue {
			return sf.FormatHints([]ShortcutHint{
				{Key: sf.formatKey("tab"), Action: "focus queue"},
				{Key: sf.formatKey(kb.Search), Action: "search"},
			})
		}
		return sf.FormatHints([]ShortcutHint{
			{Key: "q", Action: "show queue"},
			{Key: sf.formatKey(kb.Search), Action: "search"},
		})

	case PlaylistDetailView:
		if showQueue {
			return sf.FormatHints([]ShortcutHint{
				{Key: sf.formatKey("tab"), Action: "focus queue"},
				{Key: sf.formatKeys(kb.Back), Action: "home"},
			})
		}
		return sf.FormatHints([]ShortcutHint{
			{Key: "q", Action: "show queue"},
			{Key: sf.formatKeys(kb.Back), Action: "home"},
		})

	case SearchView:
		return sf.FormatHints(sf.GetSearchHints())

	default:
		if hasFocus("queue") {
			return sf.FormatHints([]ShortcutHint{
				{Key: sf.formatKey("tab"), Action: "back to main"},
				{Key: "q", Action: "hide queue"},
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