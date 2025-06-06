package ui

import (
	"time"

	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/haryoiro/yutemal/internal/structures"
	"github.com/haryoiro/yutemal/internal/systems"
	"github.com/mattn/go-runewidth"
)

func init() {
	runewidth.DefaultCondition.EastAsianWidth = false
}

type ViewState int

const (
	HomeView ViewState = iota
	PlaylistListView
	PlaylistDetailView
	SearchView
)

type Model struct {
	systems            *systems.Systems
	config             *structures.Config
	themeManager       *ThemeManager
	state              ViewState
	width              int
	height             int
	playerHeight       int
	contentHeight      int
	playerContentWidth int

	// Section-related fields
	sections            []structures.Section
	currentSectionIndex int
	selectedIndex       int
	scrollOffset        int

	// Legacy fields for compatibility
	playlists       []systems.Playlist
	currentList     []structures.Track
	currentListName string

	// Other fields
	playerState   structures.PlayerState
	searchQuery   string
	searchResults []structures.Track
	err           error
	marqueeOffset int
	marqueeTicker *time.Ticker
	lastUpdate    time.Time

	// Mouse wheel throttling
	lastScrollTime time.Time
	scrollCooldown time.Duration
}

type tickMsg time.Time
type playerUpdateMsg structures.PlayerState
type playlistsLoadedMsg []systems.Playlist
type tracksLoadedMsg []structures.Track
type sectionsLoadedMsg []structures.Section
type errorMsg error

func RunSimple(systems *systems.Systems, config *structures.Config) error {
	m := Model{
		systems:       systems,
		config:        config,
		themeManager:  NewThemeManager(config.Theme),
		state:         HomeView,
		playerHeight:  5,
		marqueeTicker: time.NewTicker(150 * time.Millisecond),
		scrollCooldown: 20 * time.Millisecond, // 50ms between scroll events
	}

	opts := []tea.ProgramOption{
		tea.WithMouseCellMotion(), // マウスイベントを有効化
	}
	p := tea.NewProgram(&m, opts...)
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadSections(),
		m.tickCmd(),
		m.listenToPlayer(),
	)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// プレイヤースタイルのフレームサイズを考慮
		playerStyle := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			Padding(0, 1)
		playerV, _ := playerStyle.GetFrameSize()

		m.contentHeight = m.height - m.playerHeight - playerV

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.MouseMsg:
		return m.handleMouseEvent(msg)

	case tickMsg:
		m.lastUpdate = time.Time(msg)
		m.marqueeOffset++
		return m, m.tickCmd()

	case playerUpdateMsg:
		m.playerState = structures.PlayerState(msg)
		return m, m.listenToPlayer()

	case sectionsLoadedMsg:
		m.sections = msg

		// Find "Your Library" section and set it as default, or use first section
		m.currentSectionIndex = 0
		for i, section := range m.sections {
			if section.ID == "library" || section.Title == "Your Library" {
				m.currentSectionIndex = i
				break
			}
		}

		m.selectedIndex = 0
		m.scrollOffset = 0
		return m, nil

	case playlistsLoadedMsg:
		m.playlists = msg
		m.selectedIndex = 0
		m.scrollOffset = 0
		return m, nil

	case tracksLoadedMsg:
		m.currentList = msg
		m.selectedIndex = 0
		m.scrollOffset = 0
		// Download all songs in the playlist when loaded
		return m, m.downloadAllSongs(msg)

	case errorMsg:
		m.err = msg
		return m, nil
	}

	return m, nil
}

func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	// スタイルを先に定義
	borderColor := lipgloss.Color(m.config.Theme.Border)
	if m.themeManager != nil {
		borderColor = lipgloss.Color(m.config.Theme.Border)
	}

	mainStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, -3)

	playerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)

	// GetFrameSize()でフレームサイズを取得
	mainV, mainH := mainStyle.GetFrameSize()
	playerV, playerH := playerStyle.GetFrameSize()

	// 実際のコンテンツ幅を計算
	contentWidth := m.width - mainH
	playerContentWidth := m.width - playerH

	// 正確な幅と高さを設定
	mainStyle = mainStyle.
		Width(contentWidth).
		Height(m.contentHeight)

	playerStyle = playerStyle.
		Width(playerContentWidth).
		Height(m.playerHeight - playerV)

	// コンテンツをレンダリング（実際の利用可能幅を渡す）
	var content string
	switch m.state {
	case HomeView:
		content = m.renderHome(contentWidth)
	case PlaylistDetailView:
		content = m.renderPlaylistDetail(contentWidth)
	case SearchView:
		content = m.renderSearch(contentWidth)
	}

	// プレイヤーに正しい幅を渡す
	m.playerContentWidth = playerContentWidth
	player := m.renderPlayer()

	// Split content by lines and ensure it fits in the content area
	contentLines := strings.Split(content, "\n")
	maxContentLines := m.contentHeight - mainV
	if len(contentLines) > maxContentLines {
		contentLines = contentLines[:maxContentLines]
	}
	content = strings.Join(contentLines, "\n")

	result := lipgloss.JoinVertical(
		lipgloss.Top,
		mainStyle.Render(content),
		playerStyle.Render(player),
	)

	return result
}

// Helper functions for key binding checks
func (m *Model) isKey(msg tea.KeyMsg, key string) bool {
	if key == "" {
		return false
	}

	// Handle special keys
	switch key {
	case "ctrl+d":
		return msg.Type == tea.KeyCtrlD
	case "space":
		return msg.Type == tea.KeySpace
	case "enter":
		return msg.Type == tea.KeyEnter
	case "esc":
		return msg.Type == tea.KeyEsc
	case "backspace":
		return msg.Type == tea.KeyBackspace
	case "tab":
		return msg.Type == tea.KeyTab
	case "shift+tab":
		return msg.Type == tea.KeyShiftTab
	case "up":
		return msg.Type == tea.KeyUp
	case "down":
		return msg.Type == tea.KeyDown
	case "left":
		return msg.Type == tea.KeyLeft
	case "right":
		return msg.Type == tea.KeyRight
	default:
		// Handle regular character keys
		return msg.Type == tea.KeyRunes && msg.String() == key
	}
}

func (m *Model) isKeyInList(msg tea.KeyMsg, keys []string) bool {
	for _, key := range keys {
		if m.isKey(msg, key) {
			return true
		}
	}
	return false
}

func (m *Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	kb := m.config.KeyBindings

	// Global controls
	if m.isKey(msg, kb.Quit) {
		return m, tea.Quit
	}

	if m.isKey(msg, kb.PlayPause) {
		m.systems.Player.SendAction(structures.PlayPauseAction{})
		return m, nil
	}

	if m.isKey(msg, kb.SeekBackward) {
		m.systems.Player.SendAction(structures.BackwardAction{})
		return m, nil
	}

	if m.isKey(msg, kb.SeekForward) {
		m.systems.Player.SendAction(structures.ForwardAction{})
		return m, nil
	}

	if m.isKeyInList(msg, kb.VolumeUp) {
		m.systems.Player.SendAction(structures.VolumeUpAction{})
		return m, nil
	}

	if m.isKeyInList(msg, kb.VolumeDown) {
		m.systems.Player.SendAction(structures.VolumeDownAction{})
		return m, nil
	}

	// Navigation
	if m.isKeyInList(msg, kb.MoveUp) {
		if m.selectedIndex > 0 {
			m.selectedIndex--
			m.adjustScroll()
		}
		return m, nil
	}

	if m.isKeyInList(msg, kb.MoveDown) {
		maxIndex := m.getMaxIndex()
		if maxIndex >= 0 && m.selectedIndex < maxIndex {
			m.selectedIndex++
			m.adjustScroll()
		}
		return m, nil
	}

	// Page Up/Down handling
	if msg.Type == tea.KeyPgUp {
		// View毎に異なる高さ調整
		visibleItems := m.contentHeight - 4
		switch m.state {
		case HomeView:
			visibleItems = m.contentHeight - 8
		case PlaylistDetailView:
			visibleItems = m.contentHeight - 4
		case SearchView:
			visibleItems = m.contentHeight - 4
		}
		if visibleItems < 1 {
			visibleItems = 1
		}

		// 1ページ分上にスクロール
		m.selectedIndex -= visibleItems
		if m.selectedIndex < 0 {
			m.selectedIndex = 0
		}
		m.adjustScroll()
		return m, nil
	}

	if msg.Type == tea.KeyPgDown {
		// View毎に異なる高さ調整
		visibleItems := m.contentHeight - 4
		switch m.state {
		case HomeView:
			visibleItems = m.contentHeight - 8
		case PlaylistDetailView:
			visibleItems = m.contentHeight - 4
		case SearchView:
			visibleItems = m.contentHeight - 4
		}
		if visibleItems < 1 {
			visibleItems = 1
		}

		// 1ページ分下にスクロール
		maxIndex := m.getMaxIndex()
		m.selectedIndex += visibleItems
		if m.selectedIndex > maxIndex {
			m.selectedIndex = maxIndex
		}
		m.adjustScroll()
		return m, nil
	}

	if m.isKeyInList(msg, kb.Select) {
		return m.handleEnter()
	}

	if m.isKeyInList(msg, kb.Back) {
		if m.state == PlaylistDetailView {
			m.state = HomeView
			m.selectedIndex = 0
			m.scrollOffset = 0
		} else if m.state == SearchView {
			m.state = HomeView
		}
		return m, nil
	}

	if m.isKey(msg, kb.NextSection) {
		if m.state == HomeView && len(m.sections) > 0 {
			m.currentSectionIndex = (m.currentSectionIndex + 1) % len(m.sections)
			m.selectedIndex = 0
			m.scrollOffset = 0
		}
		return m, nil
	}

	if m.isKey(msg, kb.PrevSection) {
		if m.state == HomeView && len(m.sections) > 0 {
			m.currentSectionIndex = (m.currentSectionIndex - 1 + len(m.sections)) % len(m.sections)
			m.selectedIndex = 0
			m.scrollOffset = 0
		}
		return m, nil
	}

	// Actions
	if m.isKey(msg, kb.Search) {
		m.state = SearchView
		m.searchQuery = ""
		m.selectedIndex = 0
		return m, nil
	}

	if m.isKey(msg, kb.Shuffle) {
		// Shuffle is not implemented in the Go version yet
		return m, nil
	}

	if m.isKey(msg, kb.RemoveTrack) {
		if m.state == PlaylistDetailView && len(m.currentList) > 0 {
			// Remove current song action
			m.systems.Player.SendAction(structures.DeleteTrackAction{})
		}
		return m, nil
	}

	if m.isKey(msg, kb.Home) {
		if m.state == PlaylistDetailView {
			m.state = HomeView
			m.selectedIndex = 0
			m.scrollOffset = 0
		}
		return m, nil
	}

	return m, nil
}

func (m *Model) getMaxIndex() int {
	switch m.state {
	case HomeView:
		if len(m.sections) > 0 && m.currentSectionIndex < len(m.sections) {
			return len(m.sections[m.currentSectionIndex].Contents) - 1
		}
		return 0
	case PlaylistListView:
		return len(m.playlists) - 1
	case PlaylistDetailView:
		return len(m.currentList) - 1
	case SearchView:
		return len(m.searchResults) - 1
	}
	return 0
}

func (m *Model) adjustScroll() {
	// View毎に異なる高さ調整
	visibleItems := m.contentHeight - 4 // Default adjustment
	switch m.state {
	case HomeView:
		visibleItems = m.contentHeight - 8 // タブとタイトル用のスペースを確保
	case PlaylistDetailView:
		visibleItems = m.contentHeight - 4
	case SearchView:
		visibleItems = m.contentHeight - 4
	}

	if visibleItems < 1 {
		visibleItems = 1
	}
	if m.selectedIndex < m.scrollOffset {
		m.scrollOffset = m.selectedIndex
	} else if m.selectedIndex >= m.scrollOffset+visibleItems {
		m.scrollOffset = m.selectedIndex - visibleItems + 1
	}
}

func (m *Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.state {
	case HomeView:
		if len(m.sections) > 0 && m.currentSectionIndex < len(m.sections) && m.selectedIndex < len(m.sections[m.currentSectionIndex].Contents) {
			content := m.sections[m.currentSectionIndex].Contents[m.selectedIndex]
			if content.Type == "playlist" && content.Playlist != nil {
				playlist := content.Playlist
				m.currentList = []structures.Track{} // Reset current list
				m.state = PlaylistDetailView
				m.currentListName = playlist.Title
				return m, m.loadPlaylistTracks(playlist.ID)
			} else if content.Type == "track" && content.Track != nil {
				track := content.Track
				m.systems.Player.SendAction(structures.CleanupAction{})
				m.systems.Player.SendAction(structures.AddTrackAction{Track: *track})
				m.systems.Player.SendAction(structures.PlayAction{})
			}
		}
	case PlaylistListView:
		if len(m.playlists) > 0 && m.selectedIndex < len(m.playlists) {
			playlist := m.playlists[m.selectedIndex]
			m.currentList = []structures.Track{} // Reset current list
			m.state = PlaylistDetailView
			m.currentListName = playlist.Title
			return m, m.loadPlaylistTracks(playlist.ID)
		}
	case PlaylistDetailView:
		if len(m.currentList) > 0 && m.selectedIndex < len(m.currentList) {
			// Clear the current queue
			m.systems.Player.SendAction(structures.CleanupAction{})

			// Add all tracks from the selected position onwards
			tracksToAdd := m.currentList[m.selectedIndex:]
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
	}
	return m, nil
}

func (m *Model) tickCmd() tea.Cmd {
	return tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *Model) listenToPlayer() tea.Cmd {
	return func() tea.Msg {
		// Get player state
		state := m.systems.Player.GetState()
		return playerUpdateMsg(state)
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

		return tracksLoadedMsg(tracks)
	}
}

func (m *Model) loadSections() tea.Cmd {
	return func() tea.Msg {
		sections, err := m.systems.API.GetSections()
		if err != nil {
			return errorMsg(err)
		}

		return sectionsLoadedMsg(sections)
	}
}

func (m *Model) downloadAllSongs(videos []structures.Track) tea.Cmd {
	return func() tea.Msg {
		// Send download requests for all videos in the playlist
		for _, video := range videos {
			m.systems.Download.QueueDownload(video)
		}
		return nil
	}
}
