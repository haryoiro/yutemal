package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/haryoiro/yutemal/internal/logger"
	"github.com/haryoiro/yutemal/internal/structures"
)

// アクション処理関連の関数

// handleEnter handles enter key press for different views.
func (m *Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.state {
	case HomeView:
		if len(m.sections) > 0 && m.currentSectionIndex < len(m.sections) && m.selectedIndex < len(m.sections[m.currentSectionIndex].Contents) {
			content := m.sections[m.currentSectionIndex].Contents[m.selectedIndex]
			if content.Type == "playlist" && content.Playlist != nil {
				playlist := content.Playlist
				logger.Debug("Opening playlist from HomeView: %s, changing state to PlaylistDetailView", playlist.Title)
				// Reset playlist view state
				m.playlistTracks = []structures.Track{}
				m.playlistName = playlist.Title
				m.playlistSelectedIndex = 0
				m.playlistScrollOffset = 0
				m.state = PlaylistDetailView
				// Keep backward compatibility
				m.currentList = []structures.Track{}
				m.currentListName = playlist.Title

				return m, m.loadPlaylistTracks(playlist.ID)
			} else if content.Type == "track" && content.Track != nil {
				track := content.Track
				// Add track to queue and play
				m.systems.Player.SendAction(structures.CleanupAction{})
				m.systems.Player.SendAction(structures.AddTrackAction{Track: *track})
				m.systems.Player.SendAction(structures.PlayAction{})
			}
		}
	case PlaylistDetailView:
		if len(m.playlistTracks) > 0 && m.playlistSelectedIndex < len(m.playlistTracks) {
			// Clear the current queue
			m.systems.Player.SendAction(structures.CleanupAction{})

			// Add all tracks from the selected position onwards
			tracksToAdd := m.playlistTracks[m.playlistSelectedIndex:]
			m.systems.Player.SendAction(structures.AddTracksToQueueAction{Tracks: tracksToAdd})

			// Start playing
			m.systems.Player.SendAction(structures.PlayAction{})
		}
	case SearchView:
		if len(m.searchResults) > 0 && m.selectedIndex < len(m.searchResults) {
			track := m.searchResults[m.selectedIndex]
			m.systems.Player.SendAction(structures.CleanupAction{})
			m.systems.Player.SendAction(structures.AddTrackAction{Track: track})
			m.systems.Player.SendAction(structures.PlayAction{})
		}
	case PlaylistListView:
		if len(m.playlists) > 0 && m.selectedIndex < len(m.playlists) {
			playlist := m.playlists[m.selectedIndex]
			m.playlistTracks = []structures.Track{}
			m.playlistName = playlist.Title
			m.playlistSelectedIndex = 0
			m.playlistScrollOffset = 0
			m.state = PlaylistDetailView
			m.currentList = []structures.Track{}
			m.currentListName = playlist.Title

			return m, m.loadPlaylistTracks(playlist.ID)
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

func (m *Model) loadSections() tea.Cmd {
	return func() tea.Msg {
		sections, err := m.systems.API.GetSections()
		if err != nil {
			playlists, err := m.systems.API.GetLibraryPlaylists()
			if err != nil {
				return errorMsg(err)
			}
			// Convert playlists to a section
			contents := make([]structures.ContentItem, len(playlists))
			for i, p := range playlists {
				contents[i] = structures.ContentItem{
					Type: "playlist",
					Playlist: &structures.Playlist{
						ID:          p.ID,
						Title:       p.Title,
						Description: p.Description,
						Thumbnail:   "",
						VideoCount:  0,
					},
				}
			}

			return sectionsLoadedMsg([]structures.Section{
				{
					ID:       "library",
					Title:    "Your Library",
					Type:     structures.SectionTypeLibraryPlaylists,
					Contents: contents,
				},
			})
		}

		return sectionsLoadedMsg(sections)
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
	// Start unified tick if any animation needs it
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
