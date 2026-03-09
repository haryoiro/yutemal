package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/haryoiro/yutemal/internal/listnav"
	"github.com/haryoiro/yutemal/internal/logger"
)

// mainListNav creates a ListNav for the current main view (playlist list or search).
func (m *Model) mainListNav() *listnav.ListNav {
	return &listnav.ListNav{
		Selected:     m.selectedIndex,
		ScrollOffset: m.scrollOffset,
		ListSize:     m.getMaxIndex() + 1,
		PageSize:     m.getVisibleItems(),
	}
}

func (m *Model) applyMainNav(nav *listnav.ListNav) {
	m.selectedIndex = nav.Selected
	m.scrollOffset = nav.ScrollOffset
}

// playlistListNav creates a ListNav for the playlist detail view.
func (m *Model) playlistListNav() *listnav.ListNav {
	return &listnav.ListNav{
		Selected:     m.playlistSelectedIndex,
		ScrollOffset: m.playlistScrollOffset,
		ListSize:     len(m.playlistTracks),
		PageSize:     max(m.contentHeight-6, 1),
	}
}

func (m *Model) applyPlaylistNav(nav *listnav.ListNav) {
	m.playlistSelectedIndex = nav.Selected
	m.playlistScrollOffset = nav.ScrollOffset
}

// queueListNav creates a ListNav for the queue.
func (m *Model) queueListNav() *listnav.ListNav {
	return &listnav.ListNav{
		Selected:     m.queueSelectedIndex,
		ScrollOffset: m.queueScrollOffset,
		ListSize:     len(m.playerState.List),
		PageSize:     m.getQueueVisibleLines(),
	}
}

func (m *Model) applyQueueNav(nav *listnav.ListNav) {
	m.queueSelectedIndex = nav.Selected
	m.queueScrollOffset = nav.ScrollOffset
}

// moveUp handles upward navigation for both main content and queue.
func (m *Model) moveUp() (tea.Model, tea.Cmd) {
	if m.queueFocused && m.showQueue {
		nav := m.queueListNav()
		nav.MoveUp()
		m.applyQueueNav(nav)
	} else {
		switch m.state {
		case PlaylistDetailView:
			nav := m.playlistListNav()
			nav.MoveUp()
			m.applyPlaylistNav(nav)
		default:
			nav := m.mainListNav()
			nav.MoveUp()
			m.applyMainNav(nav)
		}
	}
	return m, nil
}

// moveDown handles downward navigation for both main content and queue.
func (m *Model) moveDown() (tea.Model, tea.Cmd) {
	if m.queueFocused && m.showQueue {
		nav := m.queueListNav()
		nav.MoveDown()
		m.applyQueueNav(nav)
	} else {
		switch m.state {
		case PlaylistDetailView:
			nav := m.playlistListNav()
			nav.MoveDown()
			m.applyPlaylistNav(nav)
		default:
			nav := m.mainListNav()
			nav.MoveDown()
			m.applyMainNav(nav)
		}
	}
	return m, nil
}

// jumpToTop moves selection to the first item.
func (m *Model) jumpToTop() (tea.Model, tea.Cmd) {
	if m.queueFocused && m.showQueue {
		nav := m.queueListNav()
		nav.JumpToTop()
		m.applyQueueNav(nav)
	} else {
		switch m.state {
		case PlaylistDetailView:
			nav := m.playlistListNav()
			nav.JumpToTop()
			m.applyPlaylistNav(nav)
		default:
			nav := m.mainListNav()
			nav.JumpToTop()
			m.applyMainNav(nav)
		}
	}
	return m, nil
}

// jumpToBottom moves selection to the last item.
func (m *Model) jumpToBottom() (tea.Model, tea.Cmd) {
	if m.queueFocused && m.showQueue {
		nav := m.queueListNav()
		nav.JumpToBottom()
		m.applyQueueNav(nav)
	} else {
		switch m.state {
		case PlaylistDetailView:
			nav := m.playlistListNav()
			nav.JumpToBottom()
			m.applyPlaylistNav(nav)
		default:
			nav := m.mainListNav()
			nav.JumpToBottom()
			m.applyMainNav(nav)
		}
	}
	return m, nil
}

// pageUp moves selection up by one page.
func (m *Model) pageUp() (tea.Model, tea.Cmd) {
	if m.queueFocused && m.showQueue {
		nav := m.queueListNav()
		nav.PageUp()
		m.applyQueueNav(nav)
	} else {
		switch m.state {
		case PlaylistDetailView:
			nav := m.playlistListNav()
			nav.PageUp()
			m.applyPlaylistNav(nav)
		default:
			nav := m.mainListNav()
			nav.PageUp()
			m.applyMainNav(nav)
		}
	}
	return m, nil
}

// pageDown moves selection down by one page.
func (m *Model) pageDown() (tea.Model, tea.Cmd) {
	if m.queueFocused && m.showQueue {
		nav := m.queueListNav()
		nav.PageDown()
		m.applyQueueNav(nav)
	} else {
		switch m.state {
		case PlaylistDetailView:
			nav := m.playlistListNav()
			nav.PageDown()
			m.applyPlaylistNav(nav)
		default:
			nav := m.mainListNav()
			nav.PageDown()
			m.applyMainNav(nav)
		}
	}
	return m, nil
}

// navigateBack handles back navigation between views.
func (m *Model) navigateBack() (tea.Model, tea.Cmd) {
	logger.Debug("navigateBack called: current state=%s, focused pane=%d", m.state, m.getFocusedPane())

	if m.getFocusedPane() == FocusQueue {
		logger.Debug("navigateBack: Unfocusing queue, staying in current view")
		m.setFocus(FocusMain)
		return m, nil
	}

	if m.getFocusedPane() == FocusPlayer {
		logger.Debug("navigateBack: Unfocusing player, staying in current view")
		m.setFocus(FocusMain)
		return m, nil
	}

	switch m.state {
	case PlaylistDetailView:
		logger.Debug("navigateBack: Returning from PlaylistDetailView to PlaylistListView")
		m.state = PlaylistListView
	case SearchView:
		logger.Debug("navigateBack: Returning from SearchView to PlaylistListView")
		m.state = PlaylistListView
		m.searchQuery = ""
		m.searchResults = nil
		m.setFocus(FocusMain)
	case PlaylistListView:
		logger.Debug("navigateBack: Already at PlaylistListView, ignoring")
	default:
		logger.Debug("navigateBack: Unknown state %s, ignoring", m.state)
	}

	return m, nil
}

// getMaxIndex returns the maximum selectable index for the current view.
func (m *Model) getMaxIndex() int {
	switch m.state {
	case PlaylistListView:
		if len(m.playlists) > 0 {
			return len(m.playlists) - 1
		}
		return 0
	case PlaylistDetailView:
		if len(m.playlistTracks) > 0 {
			return len(m.playlistTracks) - 1
		}
		return 0
	case SearchView:
		if len(m.searchResults) > 0 {
			return len(m.searchResults) - 1
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
	queueHeight := max(contentAreaHeight/3, 5)
	return max(queueHeight-4, 1)
}

// adjustScroll adjusts the scroll offset to keep the selected item visible.
// Thin wrapper used by mouse.go.
func (m *Model) adjustScroll() {
	nav := m.mainListNav()
	nav.AdjustScroll()
	m.applyMainNav(nav)
}

// adjustPlaylistScroll adjusts the playlist scroll offset.
// Thin wrapper used by mouse.go.
func (m *Model) adjustPlaylistScroll() {
	nav := m.playlistListNav()
	nav.AdjustScroll()
	m.applyPlaylistNav(nav)
}
