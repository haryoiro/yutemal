package ui

import (
	"github.com/haryoiro/yutemal/internal/logger"
)

// FocusPane represents which pane currently has focus.
type FocusPane int

const (
	FocusMain FocusPane = iota
	FocusQueue
	FocusPlayer
	FocusSearch
)

// Focus management methods

// getFocusedPane returns the currently focused pane.
func (m *Model) getFocusedPane() FocusPane {
	if m.playerFocused {
		return FocusPlayer
	}

	if m.queueFocused && m.showQueue {
		return FocusQueue
	}

	if m.state == SearchView {
		return FocusSearch
	}

	return FocusMain
}

// setFocus sets the focus to a specific pane.
func (m *Model) setFocus(pane FocusPane) {
	oldFocus := m.getFocusedPane()

	switch pane {
	case FocusMain:
		m.queueFocused = false
		m.playerFocused = false
	case FocusQueue:
		if m.showQueue {
			m.queueFocused = true
			m.playerFocused = false
			// Initialize queue selection at current track when focusing
			if m.queueSelectedIndex < 0 || m.queueSelectedIndex >= len(m.playerState.List) {
				m.queueSelectedIndex = m.playerState.Current
			}
		}
	case FocusPlayer:
		m.queueFocused = false
		m.playerFocused = true
	case FocusSearch:
		// Search view automatically gets focus when active
		m.queueFocused = false
		m.playerFocused = false
	}

	newFocus := m.getFocusedPane()
	if oldFocus != newFocus {
		logger.Debug("Focus changed: %d -> %d (state=%s, showQueue=%t, queueFocused=%t, playerFocused=%t)",
			oldFocus, newFocus, m.state, m.showQueue, m.queueFocused, m.playerFocused)
	}
}

// cycleFocus cycles through available focus targets: Main → Queue (if visible) → Player → Main.
func (m *Model) cycleFocus() {
	current := m.getFocusedPane()

	switch current {
	case FocusMain:
		if m.showQueue {
			m.setFocus(FocusQueue)
		} else {
			m.setFocus(FocusPlayer)
		}
	case FocusQueue:
		m.setFocus(FocusPlayer)
	case FocusPlayer:
		m.setFocus(FocusMain)
	case FocusSearch:
		// Search view keeps focus until exited
	}
}

// hasFocus returns true if the specified component has focus.
func (m *Model) hasFocus(component string) bool {
	switch component {
	case "main":
		return m.getFocusedPane() == FocusMain
	case "queue":
		return m.getFocusedPane() == FocusQueue
	case "player":
		return m.getFocusedPane() == FocusPlayer
	case "search":
		return m.getFocusedPane() == FocusSearch
	case "playlist":
		return m.state == PlaylistDetailView && m.getFocusedPane() == FocusMain
	case "playlistList":
		return m.state == PlaylistListView && m.getFocusedPane() == FocusMain
	default:
		return false
	}
}


// getFocusHelpText returns help text for the current focus state.
func (m *Model) getFocusHelpText() string {
	return m.shortcutFormatter.GetContextualHints(m.state, m.showQueue, m.hasFocus)
}
