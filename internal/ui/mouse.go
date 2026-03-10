package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/haryoiro/yutemal/internal/structures"
)

// マウスイベントのハンドリング.
func (m *Model) handleMouseEvent(mouse tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch mouse.Action {
	case tea.MouseActionPress:
		if mouse.Button == tea.MouseButtonLeft {
			return m.handleMouseClick(mouse.X, mouse.Y)
		}

		if mouse.Button == tea.MouseButtonWheelUp {
			return m.handleScrollUp()
		}

		if mouse.Button == tea.MouseButtonWheelDown {
			return m.handleScrollDown()
		}
	}

	return m, nil
}

// マウスクリックの処理.
func (m *Model) handleMouseClick(x, y int) (tea.Model, tea.Cmd) {
	playerAreaStart := m.height - m.playerHeight

	if y >= playerAreaStart {
		return m.handlePlayerClick(x, y-playerAreaStart)
	}

	return m.handleContentClick(x, y)
}

// プレイヤー部分のクリック処理.
func (m *Model) handlePlayerClick(x, y int) (tea.Model, tea.Cmd) {
	contentX := x - 1
	if contentX < 0 {
		return m, nil
	}

	adjustedY := y + 2
	if adjustedY == 3 && m.playerState.TotalTime > 0 {
		timeDisplayWidth := 6
		progressBarStart := timeDisplayWidth

		barWidth := m.playerContentWidth - (timeDisplayWidth * 2)
		if barWidth <= 0 {
			return m, nil
		}

		if contentX >= progressBarStart && contentX < progressBarStart+barWidth {
			clickPos := contentX - progressBarStart

			progress := float64(clickPos) / float64(barWidth)
			if progress < 0 {
				progress = 0
			} else if progress > 1 {
				progress = 1
			}

			seekPos := time.Duration(float64(m.playerState.TotalTime) * progress)
			m.systems.Player.SendAction(structures.SeekAction{Position: seekPos})
		}
	}

	return m, nil
}

// コンテンツ部分のクリック処理.
func (m *Model) handleContentClick(x, y int) (tea.Model, tea.Cmd) {
	contentY := y - 1

	switch m.state {
	case PlaylistDetailView:
		listStartY := 4
		relativeY := contentY - listStartY

		if relativeY >= 0 && relativeY < m.contentHeight {
			clickedIndex := m.playlistScrollOffset + relativeY

			if clickedIndex >= 0 && clickedIndex < len(m.playlistTracks) {
				m.playlistSelectedIndex = clickedIndex
				return m.playSelectedTrack()
			}
		}

	case SearchView:
		listStartY := 3
		relativeY := contentY - listStartY

		if relativeY >= 0 && relativeY < m.contentHeight {
			clickedIndex := m.scrollOffset + relativeY

			if clickedIndex >= 0 && clickedIndex < len(m.searchResults) {
				m.selectedIndex = clickedIndex
				return m.playSelectedTrack()
			}
		}

	case PlaylistListView:
		// タイトル行(2行: header + shortcuts) + 空行
		listStartY := 3
		relativeY := contentY - listStartY

		if relativeY >= 0 && relativeY < m.contentHeight {
			clickedIndex := m.scrollOffset + relativeY

			if clickedIndex >= 0 && clickedIndex < len(m.playlists) {
				m.selectedIndex = clickedIndex
				playlist := m.playlists[m.selectedIndex]
				m.playlistTracks = []structures.Track{}
				m.playlistName = playlist.Title
				m.playlistSelectedIndex = 0
				m.playlistScrollOffset = 0
				m.state = PlaylistDetailView

				return m, m.loadPlaylistTracks(playlist.ID)
			}
		}
	}

	return m, nil
}

// スクロールアップの処理.
func (m *Model) handleScrollUp() (tea.Model, tea.Cmd) {
	now := time.Now()
	if now.Sub(m.lastScrollTime) < m.scrollCooldown {
		return m, nil
	}

	m.lastScrollTime = now

	if m.queueFocused && m.showQueue {
		if m.queueSelectedIndex > 0 {
			m.queueSelectedIndex--
			if m.queueSelectedIndex < m.queueScrollOffset {
				m.queueScrollOffset = m.queueSelectedIndex
			}
		}

		return m, nil
	}

	switch m.state {
	case PlaylistDetailView:
		if m.playlistSelectedIndex > 0 {
			m.playlistSelectedIndex--
			m.adjustPlaylistScroll()
		}
	default:
		if m.selectedIndex > 0 {
			m.selectedIndex--
			m.adjustScroll()
		}
	}

	return m, nil
}

// スクロールダウンの処理.
func (m *Model) handleScrollDown() (tea.Model, tea.Cmd) {
	now := time.Now()
	if now.Sub(m.lastScrollTime) < m.scrollCooldown {
		return m, nil
	}

	m.lastScrollTime = now

	if m.queueFocused && m.showQueue {
		maxQueueIndex := len(m.playerState.List) - 1
		if m.queueSelectedIndex < maxQueueIndex {
			m.queueSelectedIndex++
			contentAreaHeight := m.height - m.playerHeight

			queueHeight := max(contentAreaHeight/3, 5)

			visibleLines := max(queueHeight-4, 1)

			if m.queueSelectedIndex >= m.queueScrollOffset+visibleLines {
				m.queueScrollOffset = m.queueSelectedIndex - visibleLines + 1
			}
		}

		return m, nil
	}

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

	return m, nil
}

// 選択されたトラックを再生.
func (m *Model) playSelectedTrack() (tea.Model, tea.Cmd) {
	switch m.state {
	case PlaylistDetailView:
		if len(m.playlistTracks) > 0 && m.playlistSelectedIndex < len(m.playlistTracks) {
			m.systems.Player.SendAction(structures.CleanupAction{})
			tracksToAdd := m.playlistTracks[m.playlistSelectedIndex:]
			m.systems.Player.SendAction(structures.AddTracksToQueueAction{Tracks: tracksToAdd})
			m.systems.Player.SendAction(structures.PlayAction{})
		}
	case SearchView:
		if len(m.searchResults) > 0 && m.selectedIndex < len(m.searchResults) {
			track := m.searchResults[m.selectedIndex]
			m.systems.Player.SendAction(structures.CleanupAction{})
			m.systems.Player.SendAction(structures.AddTrackAction{Track: track})
			m.systems.Player.SendAction(structures.PlayAction{})
		}
	}

	return m, nil
}
