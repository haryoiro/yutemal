package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/haryoiro/yutemal/internal/logger"
	"github.com/haryoiro/yutemal/internal/structures"
)

// プレイヤー操作関連の共通処理

// togglePlayPause toggles play/pause state
func (m *Model) togglePlayPause() (tea.Model, tea.Cmd) {
	m.systems.Player.SendAction(structures.PlayPauseAction{})
	return m, nil
}

// volumeUp increases the volume
func (m *Model) volumeUp() (tea.Model, tea.Cmd) {
	m.systems.Player.SendAction(structures.VolumeUpAction{})
	return m, nil
}

// volumeDown decreases the volume
func (m *Model) volumeDown() (tea.Model, tea.Cmd) {
	m.systems.Player.SendAction(structures.VolumeDownAction{})
	return m, nil
}

// seekForward seeks forward in the current track
func (m *Model) seekForward() (tea.Model, tea.Cmd) {
	m.systems.Player.SendAction(structures.ForwardAction{})
	return m, nil
}

// seekBackward seeks backward in the current track
func (m *Model) seekBackward() (tea.Model, tea.Cmd) {
	m.systems.Player.SendAction(structures.BackwardAction{})
	return m, nil
}

// shuffleQueue shuffles the current queue
func (m *Model) shuffleQueue() (tea.Model, tea.Cmd) {
	m.systems.Player.SendAction(structures.ShuffleQueueAction{})
	return m, nil
}

// removeTrack handles track removal from queue or current view
func (m *Model) removeTrack() (tea.Model, tea.Cmd) {
	if m.queueFocused && m.showQueue {
		// Remove selected track from queue
		if m.queueSelectedIndex >= 0 && m.queueSelectedIndex < len(m.playerState.List) {
			m.systems.Player.SendAction(structures.DeleteTrackAtIndexAction{Index: m.queueSelectedIndex})
			// Adjust selection after deletion
			maxIndex := len(m.playerState.List) - 2 // -2 because we're removing one
			if m.queueSelectedIndex > maxIndex && m.queueSelectedIndex > 0 {
				m.queueSelectedIndex--
			}
		}
	} else if m.state == PlaylistDetailView && len(m.currentList) > 0 {
		// Remove current song action
		m.systems.Player.SendAction(structures.DeleteTrackAction{})
	}
	return m, nil
}

// toggleQueue toggles the queue display
func (m *Model) toggleQueue() (tea.Model, tea.Cmd) {
	m.showQueue = !m.showQueue
	m.queueScrollOffset = 0
	if m.showQueue {
		// When opening queue, automatically focus it
		m.setFocus(FocusQueue)
		logger.Debug("toggleQueue: Queue shown, focus set to queue")
	} else {
		// When closing queue, return focus to main
		m.setFocus(FocusMain)
		logger.Debug("toggleQueue: Queue hidden, focus returned to main")
	}
	return m, nil
}

// toggleQueueFocus toggles focus between main content and queue
func (m *Model) toggleQueueFocus() (tea.Model, tea.Cmd) {
	if m.showQueue {
		// Toggle focus based on current state
		if m.getFocusedPane() == FocusQueue {
			m.setFocus(FocusMain)
		} else {
			m.setFocus(FocusQueue)
		}
	}
	return m, nil
}

// handleQueueSelection plays the selected track in the queue
func (m *Model) handleQueueSelection() (tea.Model, tea.Cmd) {
	if m.queueFocused && m.showQueue {
		// Play selected track in queue
		if m.queueSelectedIndex >= 0 && m.queueSelectedIndex < len(m.playerState.List) {
			// Jump to selected track
			if m.queueSelectedIndex < m.playerState.Current {
				// Jump backward
				skipCount := m.playerState.Current - m.queueSelectedIndex
				for i := 0; i < skipCount; i++ {
					m.systems.Player.SendAction(structures.PreviousAction{})
				}
			} else if m.queueSelectedIndex > m.playerState.Current {
				// Jump forward
				skipCount := m.queueSelectedIndex - m.playerState.Current
				for i := 0; i < skipCount; i++ {
					m.systems.Player.SendAction(structures.NextAction{})
				}
			}
		}
		return m, nil
	}
	return m.handleEnter()
}
