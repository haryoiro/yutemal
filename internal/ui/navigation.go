package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/haryoiro/yutemal/internal/logger"
)

// ナビゲーション関連の共通処理

// moveUp handles upward navigation for both main content and queue.
func (m *Model) moveUp() (tea.Model, tea.Cmd) {
	if m.queueFocused && m.showQueue {
		// Navigate in queue
		if m.queueSelectedIndex > 0 {
			m.queueSelectedIndex--
			m.adjustQueueScroll()
		}
	} else {
		// Navigate in main content
		switch m.state {
		case HomeView:
			if m.selectedIndex > 0 {
				m.selectedIndex--
				m.adjustScroll()
			}
		case PlaylistDetailView:
			if m.playlistSelectedIndex > 0 {
				m.playlistSelectedIndex--
				m.adjustPlaylistScroll()
			}
		case SearchView, PlaylistListView:
			if m.selectedIndex > 0 {
				m.selectedIndex--
				m.adjustScroll()
			}
		}
	}

	return m, nil
}

// moveDown handles downward navigation for both main content and queue.
func (m *Model) moveDown() (tea.Model, tea.Cmd) {
	if m.queueFocused && m.showQueue {
		// Navigate in queue
		if m.queueSelectedIndex < len(m.playerState.List)-1 {
			m.queueSelectedIndex++
			m.adjustQueueScroll()
		}
	} else {
		// Navigate in main content
		switch m.state {
		case PlaylistDetailView:
			if m.playlistSelectedIndex < len(m.playlistTracks)-1 {
				m.playlistSelectedIndex++
				m.adjustPlaylistScroll()
			}
		default:
			maxIndex := m.getMaxIndex()
			if m.selectedIndex < maxIndex {
				m.selectedIndex++
				m.adjustScroll()
			}
		}
	}

	return m, nil
}

// jumpToTop moves selection to the first item.
func (m *Model) jumpToTop() (tea.Model, tea.Cmd) {
	if m.queueFocused && m.showQueue {
		m.queueSelectedIndex = 0
		m.queueScrollOffset = 0
	} else {
		switch m.state {
		case PlaylistDetailView:
			m.playlistSelectedIndex = 0
			m.playlistScrollOffset = 0
		default:
			m.selectedIndex = 0
			m.scrollOffset = 0
		}
	}

	return m, nil
}

// jumpToBottom moves selection to the last item.
func (m *Model) jumpToBottom() (tea.Model, tea.Cmd) {
	if m.queueFocused && m.showQueue {
		m.queueSelectedIndex = len(m.playerState.List) - 1
		m.adjustQueueScroll()
	} else {
		switch m.state {
		case PlaylistDetailView:
			m.playlistSelectedIndex = len(m.playlistTracks) - 1
			m.adjustPlaylistScroll()
		default:
			maxIndex := m.getMaxIndex()
			m.selectedIndex = maxIndex
			m.adjustScroll()
		}
	}

	return m, nil
}

// pageUp moves selection up by one page.
func (m *Model) pageUp() (tea.Model, tea.Cmd) {
	if m.queueFocused && m.showQueue {
		visibleLines := m.getQueueVisibleLines()

		m.queueSelectedIndex -= visibleLines
		if m.queueSelectedIndex < 0 {
			m.queueSelectedIndex = 0
		}

		m.adjustQueueScroll()
	} else {
		visibleItems := m.getVisibleItems()

		switch m.state {
		case PlaylistDetailView:
			m.playlistSelectedIndex -= visibleItems
			if m.playlistSelectedIndex < 0 {
				m.playlistSelectedIndex = 0
			}

			m.adjustPlaylistScroll()
		default:
			m.selectedIndex -= visibleItems
			if m.selectedIndex < 0 {
				m.selectedIndex = 0
			}

			m.adjustScroll()
		}
	}

	return m, nil
}

// pageDown moves selection down by one page.
func (m *Model) pageDown() (tea.Model, tea.Cmd) {
	if m.queueFocused && m.showQueue {
		visibleLines := m.getQueueVisibleLines()
		m.queueSelectedIndex += visibleLines
		maxIndex := len(m.playerState.List) - 1

		if m.queueSelectedIndex > maxIndex {
			m.queueSelectedIndex = maxIndex
		}

		m.adjustQueueScroll()
	} else {
		visibleItems := m.getVisibleItems()

		switch m.state {
		case PlaylistDetailView:
			m.playlistSelectedIndex += visibleItems
			if m.playlistSelectedIndex >= len(m.playlistTracks) {
				m.playlistSelectedIndex = len(m.playlistTracks) - 1
			}

			m.adjustPlaylistScroll()
		default:
			maxIndex := m.getMaxIndex()
			m.selectedIndex += visibleItems

			if m.selectedIndex > maxIndex {
				m.selectedIndex = maxIndex
			}

			m.adjustScroll()
		}
	}

	return m, nil
}

// navigateBack handles back navigation between views.
func (m *Model) navigateBack() (tea.Model, tea.Cmd) {
	logger.Debug("navigateBack called: current state=%s, focused pane=%d", m.state, m.getFocusedPane())

	// If queue is focused, just unfocus it without changing views
	if m.getFocusedPane() == FocusQueue {
		logger.Debug("navigateBack: Unfocusing queue, staying in current view")
		m.setFocus(FocusMain)

		return m, nil
	}

	// Only change views if we're in the main focus area
	switch m.state {
	case PlaylistDetailView:
		// Return to HomeView, keeping the section selection
		logger.Debug("navigateBack: Returning from PlaylistDetailView to HomeView")

		m.state = HomeView
		// Don't reset the selectedIndex and scrollOffset for HomeView
		// so user returns to where they were
	case SearchView:
		logger.Debug("navigateBack: Returning from SearchView to HomeView")

		m.state = HomeView
		m.setFocus(FocusMain)
	case HomeView:
		// Already at home, do nothing
		logger.Debug("navigateBack: Already at HomeView, ignoring")
	default:
		logger.Debug("navigateBack: Unknown state %s, ignoring", m.state)
	}

	return m, nil
}

// Helper methods

// getMaxIndex returns the maximum selectable index for the current view.
func (m *Model) getMaxIndex() int {
	switch m.state {
	case HomeView:
		if m.currentSectionIndex < len(m.sections) && len(m.sections[m.currentSectionIndex].Contents) > 0 {
			return len(m.sections[m.currentSectionIndex].Contents) - 1
		}

		return 0
	case PlaylistDetailView:
		if len(m.currentList) > 0 {
			return len(m.currentList) - 1
		}

		return 0
	case SearchView:
		if len(m.searchResults) > 0 {
			return len(m.searchResults) - 1
		}

		return 0
	case PlaylistListView:
		if len(m.playlists) > 0 {
			return len(m.playlists) - 1
		}

		return 0
	default:
		return 0
	}
}

// getVisibleItems returns the number of visible items in the content area.
func (m *Model) getVisibleItems() int {
	if m.contentHeight < 1 {
		return 1
	}

	return m.contentHeight
}

// getQueueVisibleLines returns the number of visible lines in the queue.
func (m *Model) getQueueVisibleLines() int {
	contentAreaHeight := m.height - m.playerHeight

	queueHeight := contentAreaHeight / 3
	if queueHeight < 5 {
		queueHeight = 5
	}

	visibleLines := queueHeight - 4 // Header, spacing, and scroll indicator
	if visibleLines < 1 {
		visibleLines = 1
	}

	return visibleLines
}

// adjustScroll adjusts the scroll offset to keep the selected item visible.
func (m *Model) adjustScroll() {
	visibleItems := m.getVisibleItems()

	if m.selectedIndex < m.scrollOffset {
		m.scrollOffset = m.selectedIndex
	} else if m.selectedIndex >= m.scrollOffset+visibleItems {
		m.scrollOffset = m.selectedIndex - visibleItems + 1
	}
}

// adjustQueueScroll adjusts the queue scroll offset to keep the selected item visible.
func (m *Model) adjustQueueScroll() {
	visibleLines := m.getQueueVisibleLines()

	if m.queueSelectedIndex < m.queueScrollOffset {
		m.queueScrollOffset = m.queueSelectedIndex
	} else if m.queueSelectedIndex >= m.queueScrollOffset+visibleLines {
		m.queueScrollOffset = m.queueSelectedIndex - visibleLines + 1
	}
}

// adjustPlaylistScroll adjusts the playlist scroll offset to keep the selected item visible.
func (m *Model) adjustPlaylistScroll() {
	// Use same calculation as renderPlaylistDetail
	visibleItems := m.contentHeight - 6 // Header and footer space
	if visibleItems < 1 {
		visibleItems = 1
	}

	if m.playlistSelectedIndex < m.playlistScrollOffset {
		m.playlistScrollOffset = m.playlistSelectedIndex
	} else if m.playlistSelectedIndex >= m.playlistScrollOffset+visibleItems {
		m.playlistScrollOffset = m.playlistSelectedIndex - visibleItems + 1
	}
}
