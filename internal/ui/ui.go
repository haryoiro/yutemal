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

	// Queue display
	showQueue         bool
	queueWidth        int
	queueScrollOffset int
	queueFocused      bool
	queueSelectedIndex int

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

	// Key repeat prevention
	keyDebouncer *KeyDebouncer
}

type tickMsg time.Time
type playerUpdateMsg structures.PlayerState
type playlistsLoadedMsg []systems.Playlist
type tracksLoadedMsg []structures.Track
type sectionsLoadedMsg []structures.Section
type errorMsg error

func RunSimple(systems *systems.Systems, config *structures.Config) error {
	m := Model{
		systems:        systems,
		config:         config,
		themeManager:   NewThemeManager(config.Theme),
		state:          HomeView,
		playerHeight:   5,
		marqueeTicker:  time.NewTicker(150 * time.Millisecond),
		scrollCooldown: 20 * time.Millisecond, // 50ms between scroll events
		keyDebouncer:   NewKeyDebouncer(),
	}

	opts := []tea.ProgramOption{
		tea.WithMouseCellMotion(), // マウスイベントを有効化
		tea.WithAltScreen(),       // Use alternate screen
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
		// Auto-scroll queue to show current track when it changes
		if m.showQueue && len(m.playerState.List) > 0 {
			visibleLines := m.contentHeight - 4
			if visibleLines < 1 {
				visibleLines = 1
			}
			// Check if current track is out of view
			if m.playerState.Current < m.queueScrollOffset ||
			   m.playerState.Current >= m.queueScrollOffset+visibleLines {
				// Center the current track in the view
				m.queueScrollOffset = m.playerState.Current - visibleLines/2
				if m.queueScrollOffset < 0 {
					m.queueScrollOffset = 0
				}
				maxScrollOffset := len(m.playerState.List) - visibleLines
				if maxScrollOffset < 0 {
					maxScrollOffset = 0
				}
				if m.queueScrollOffset > maxScrollOffset {
					m.queueScrollOffset = maxScrollOffset
				}
			}
		}
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

	queueStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)

	// GetFrameSize()でフレームサイズを取得
	mainV, mainH := mainStyle.GetFrameSize()
	playerV, playerH := playerStyle.GetFrameSize()
	queueV, queueH := queueStyle.GetFrameSize()

	// Queue width calculation
	if m.showQueue {
		m.queueWidth = m.width / 3 // 33% of width
		if m.queueWidth < 40 {
			m.queueWidth = 40 // Minimum width
		}
		if m.queueWidth > 80 {
			m.queueWidth = 80 // Maximum width
		}
	} else {
		m.queueWidth = 0
	}

	// 実際のコンテンツ幅を計算
	mainContentWidth := m.width - mainH - m.queueWidth
	queueContentWidth := m.queueWidth - queueH
	playerContentWidth := m.width - playerH

	// 正確な幅と高さを設定
	mainStyle = mainStyle.
		Width(mainContentWidth).
		Height(m.contentHeight)

	queueStyle = queueStyle.
		Width(queueContentWidth).
		Height(m.contentHeight)

	playerStyle = playerStyle.
		Width(playerContentWidth).
		Height(m.playerHeight - playerV)

	// コンテンツをレンダリング（実際の利用可能幅を渡す）
	var content string
	switch m.state {
	case HomeView:
		content = m.renderHome(mainContentWidth)
	case PlaylistDetailView:
		content = m.renderPlaylistDetail(mainContentWidth)
	case SearchView:
		content = m.renderSearch(mainContentWidth)
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

	// Render main content and queue side by side if queue is shown
	var topContent string
	if m.showQueue {
		queue := m.renderQueue(queueContentWidth, m.contentHeight-queueV)
		topContent = lipgloss.JoinHorizontal(
			lipgloss.Left,
			mainStyle.Render(content),
			queueStyle.Render(queue),
		)
	} else {
		topContent = mainStyle.Render(content)
	}

	result := lipgloss.JoinVertical(
		lipgloss.Top,
		topContent,
		playerStyle.Render(player),
	)

	return result
}

// Key binding functions moved to keybindings.go

// handleKeyPress has been moved to keybindings.go

// Navigation helper functions moved to navigation.go

// Action handler functions moved to actions.go

// downloadAllSongs has been moved to actions.go
