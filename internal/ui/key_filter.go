package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// getKeyString converts a tea.KeyMsg to a unique string identifier
func getKeyString(msg tea.KeyMsg) string {
	// For special keys, use the type
	switch msg.Type {
	case tea.KeyUp:
		return "up"
	case tea.KeyDown:
		return "down"
	case tea.KeyLeft:
		return "left"
	case tea.KeyRight:
		return "right"
	case tea.KeyPgUp:
		return "pgup"
	case tea.KeyPgDown:
		return "pgdown"
	case tea.KeyHome:
		return "home"
	case tea.KeyEnd:
		return "end"
	case tea.KeyEnter:
		return "enter"
	case tea.KeySpace:
		return "space"
	case tea.KeyTab:
		return "tab"
	case tea.KeyBackspace:
		return "backspace"
	case tea.KeyDelete:
		return "delete"
	case tea.KeyEsc:
		return "esc"
	default:
		// For regular keys, use the string representation
		return msg.String()
	}
}

// shouldProcessKey determines if a key press should be processed based on context and debouncing
func (m *Model) shouldProcessKey(keyStr string, msg tea.KeyMsg) bool {
	// Always allow certain keys without debouncing
	switch keyStr {
	case "q", "enter", "space", "tab", "backspace", "esc":
		// These keys should not be rate-limited
		return true
	case "up", "down", "left", "right", "pgup", "pgdown":
		// Navigation keys need debouncing to prevent flooding
		return m.keyDebouncer.ShouldProcess(keyStr)
	}

	// For search view, allow all character input without debouncing
	if m.state == SearchView && msg.Type == tea.KeyRunes {
		return true
	}

	// Volume controls need some debouncing but less restrictive
	if keyStr == "+" || keyStr == "-" || keyStr == "=" {
		return m.keyDebouncer.ShouldProcess("volume")
	}

	// All other keys don't need debouncing
	return true
}

// isNavigationKey returns true if the key is a navigation key
func isNavigationKey(keyStr string) bool {
	switch keyStr {
	case "up", "down", "left", "right", "pgup", "pgdown", "home", "end":
		return true
	case "j", "k", "h", "l": // Vim-style navigation
		return true
	case "g", "G": // Jump to top/bottom
		return true
	}
	return false
}

