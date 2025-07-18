package ui

import (
	"github.com/haryoiro/yutemal/internal/logger"
)

// FocusPane represents which pane currently has focus.
type FocusPane int

const (
	FocusMain FocusPane = iota
	FocusQueue
	FocusSearch
)

// Focus management methods

// getFocusedPane returns the currently focused pane.
func (m *Model) getFocusedPane() FocusPane {
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
	case FocusQueue:
		if m.showQueue {
			m.queueFocused = true
			// Initialize queue selection at current track when focusing
			if m.queueSelectedIndex < 0 || m.queueSelectedIndex >= len(m.playerState.List) {
				m.queueSelectedIndex = m.playerState.Current
			}
		}
	case FocusSearch:
		// Search view automatically gets focus when active
		m.queueFocused = false
	}

	newFocus := m.getFocusedPane()
	if oldFocus != newFocus {
		logger.Debug("Focus changed: %d -> %d (state=%s, showQueue=%t, queueFocused=%t)",
			oldFocus, newFocus, m.state, m.showQueue, m.queueFocused)
	}
}

// cycleFocus cycles through available focus targets.
func (m *Model) cycleFocus(forward bool) {
	current := m.getFocusedPane()

	switch current {
	case FocusMain:
		if forward && m.showQueue {
			m.setFocus(FocusQueue)
		}
	case FocusQueue:
		if !forward || !m.showQueue {
			m.setFocus(FocusMain)
		}
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
	case "search":
		return m.getFocusedPane() == FocusSearch
	case "playlist":
		return m.state == PlaylistDetailView && m.getFocusedPane() == FocusMain
	case "home":
		return m.state == HomeView && m.getFocusedPane() == FocusMain
	default:
		return false
	}
}

// canNavigate returns true if navigation is allowed in the current focus state.
func (m *Model) canNavigate() bool {
	focus := m.getFocusedPane()
	switch focus {
	case FocusMain:
		return true
	case FocusQueue:
		return true
	case FocusSearch:
		// Navigation is limited in search view
		return false
	}

	return false
}

// getFocusHelpText returns help text for the current focus state.
func (m *Model) getFocusHelpText() string {
	return m.shortcutFormatter.GetContextualHints(m.state, m.showQueue, m.hasFocus)
}
