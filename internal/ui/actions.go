package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/haryoiro/yutemal/internal/logger"
	"github.com/haryoiro/yutemal/internal/structures"
)

// handleEnter handles enter key press for different views.
func (m *Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.state {
	case PlaylistListView:
		if len(m.playlists) > 0 && m.selectedIndex < len(m.playlists) {
			playlist := m.playlists[m.selectedIndex]
			logger.Debug("Opening playlist: %s, changing state to PlaylistDetailView", playlist.Title)
			m.playlistTracks = []structures.Track{}
			m.playlistName = playlist.Title
			m.playlistSelectedIndex = 0
			m.playlistScrollOffset = 0
			m.state = PlaylistDetailView

			return m, m.loadPlaylistTracks(playlist.ID)
		}
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

func (m *Model) performSearch() tea.Cmd {
	return func() tea.Msg {
		results, err := m.systems.API.Search(strings.TrimSpace(m.searchQuery))
		if err != nil {
			return errorMsg(err)
		}

		return tracksLoadedMsg(results.Tracks)
	}
}

func (m *Model) loadPlaylists() tea.Cmd {
	return func() tea.Msg {
		playlists, err := m.systems.API.GetLibraryPlaylists()
		if err != nil {
			return errorMsg(err)
		}

		return playlistsLoadedMsg(playlists)
	}
}

func (m *Model) loadPlaylistTracks(playlistID string) tea.Cmd {
	return func() tea.Msg {
		tracks, err := m.systems.API.GetPlaylistTracks(playlistID)
		if err != nil {
			return errorMsg(err)
		}

		result := make([]structures.Track, len(tracks))
		for i, t := range tracks {
			result[i] = structures.Track{
				TrackID:     t.TrackID,
				Title:       t.Title,
				Artists:     t.Artists,
				Thumbnail:   t.Thumbnail,
				Duration:    t.Duration,
				IsAvailable: t.IsAvailable,
				IsExplicit:  t.IsExplicit,
			}
		}

		return tracksLoadedMsg(result)
	}
}

func (m *Model) downloadAllSongs(tracks []structures.Track) tea.Cmd {
	return func() tea.Msg {
		for _, track := range tracks {
			m.systems.Download.QueueDownload(track)
		}

		return nil
	}
}

func (m *Model) checkMarqueeCmd() tea.Cmd {
	if m.shouldTick() && !m.tickActive {
		m.tickActive = true
		return m.unifiedTickCmd()
	}
	return nil
}

func (m *Model) listenToPlayer() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(50 * time.Millisecond)

		state := m.systems.Player.GetState()

		return playerUpdateMsg(state)
	}
}
