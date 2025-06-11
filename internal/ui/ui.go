package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/haryoiro/yutemal/internal/logger"
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

func (v ViewState) String() string {
	switch v {
	case HomeView:
		return "HomeView"
	case PlaylistListView:
		return "PlaylistListView"
	case PlaylistDetailView:
		return "PlaylistDetailView"
	case SearchView:
		return "SearchView"
	default:
		return "Unknown"
	}
}

type Model struct {
	systems            *systems.Systems
	config             *structures.Config
	themeManager       *ThemeManager
	shortcutFormatter  *ShortcutFormatter
	state              ViewState
	width              int
	height             int
	playerHeight       int
	contentHeight      int
	playerContentWidth int

	// Section-related fields (HomeView)
	sections            []structures.Section
	currentSectionIndex int
	selectedIndex       int
	scrollOffset        int

	// PlaylistDetailView fields
	playlistTracks      []structures.Track
	playlistName        string
	playlistSelectedIndex int
	playlistScrollOffset  int

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
	needsMarquee  bool // Track if marquee is needed for current content

	// Mouse wheel throttling
	lastScrollTime time.Time
	scrollCooldown time.Duration

	// Key repeat prevention
	keyDebouncer *KeyDebouncer
	lastBackKeyTime *time.Time // Strict debouncing for back navigation keys

	// Debug state tracking
	debugStateChanges []string
	debugMessageLog   []string
	showDebugInfo     bool // デバッグ情報表示フラグ
}

type tickMsg time.Time
type playerUpdateMsg structures.PlayerState
type playlistsLoadedMsg []systems.Playlist
type tracksLoadedMsg []structures.Track
type sectionsLoadedMsg []structures.Section
type errorMsg error

func RunSimple(systems *systems.Systems, config *structures.Config) error {
	m := Model{
		systems:           systems,
		config:            config,
		themeManager:      NewThemeManager(config.Theme),
		shortcutFormatter: NewShortcutFormatter(config),
		state:             HomeView,
		playerHeight:      5,
		marqueeTicker:     time.NewTicker(500 * time.Millisecond), // Match the tickCmd frequency
		scrollCooldown:    20 * time.Millisecond, // 50ms between scroll events
		keyDebouncer:      NewKeyDebouncer(),
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
	logger.Debug("Init called, starting with state: %v", m.state)
	return tea.Batch(
		m.loadSections(),
		// Don't start ticker initially - it will start when needed
		m.listenToPlayer(),
	)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// デバッグ用メッセージログ記録
	oldState := m.state

	// メッセージ種別によるログ記録
	msgType := ""
	switch msg := msg.(type) {
	case tea.KeyMsg:
		msgType = "KeyMsg: " + msg.String()
		logger.Debug("KeyMsg received: %v, current state: %v", msg, m.state)
	case sectionsLoadedMsg:
		msgType = "sectionsLoadedMsg"
		logger.Debug("sectionsLoadedMsg received, current state: %v", m.state)
	case tracksLoadedMsg:
		msgType = "tracksLoadedMsg"
		logger.Debug("tracksLoadedMsg received, current state: %v", m.state)
	case errorMsg:
		msgType = "errorMsg"
		logger.Debug("errorMsg received: %v, current state: %v", msg, m.state)
	case tickMsg:
		// Tickメッセージは多すぎるので記録しない
	default:
		msgType = "other"
	}

	if msgType != "" && msgType != "other" {
		// デバッグメッセージをリングバッファに記録
		m.debugMessageLog = append(m.debugMessageLog, msgType + " @ " + m.state.String())
		if len(m.debugMessageLog) > 20 {
			m.debugMessageLog = m.debugMessageLog[1:]
		}
	}

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
		if m.needsMarquee {
			m.marqueeOffset++
			return m, m.tickCmd()
		}
		return m, nil

	case playerUpdateMsg:
		m.playerState = structures.PlayerState(msg)
		if m.showQueue && !m.queueFocused && len(m.playerState.List) > 0 {
			visibleLines := m.contentHeight - 4
			if visibleLines < 1 {
				visibleLines = 1
			}
			if m.playerState.Current < m.queueScrollOffset ||
			   m.playerState.Current >= m.queueScrollOffset+visibleLines {
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

		return m, tea.Batch(
			m.listenToPlayer(),
			m.checkMarqueeCmd(),
		)

	case sectionsLoadedMsg:
		if m.state == HomeView {
			m.sections = msg

			m.currentSectionIndex = 0
			for i, section := range m.sections {
				if section.ID == "library" || section.Title == "Your Library" {
					m.currentSectionIndex = i
					break
				}
			}

			m.selectedIndex = 0
			m.scrollOffset = 0
		}
		return m, nil

	case playlistsLoadedMsg:
		m.playlists = msg
		m.selectedIndex = 0
		m.scrollOffset = 0
		return m, nil

	case tracksLoadedMsg:
		m.playlistTracks = msg
		m.currentList = msg
		if m.state == PlaylistDetailView {
			// Already reset in handleEnter, but ensure consistency
			if m.playlistSelectedIndex >= len(msg) {
				m.playlistSelectedIndex = 0
			}
			if m.playlistScrollOffset > 0 && m.playlistScrollOffset >= len(msg) {
				m.playlistScrollOffset = 0
			}
		} else {
			// For other views, use the general indices
			m.selectedIndex = 0
			m.scrollOffset = 0
		}
		// Download all songs in the playlist when loaded
		return m, m.downloadAllSongs(msg)

	case errorMsg:
		m.err = msg
		return m, nil
	}

	// 状態変更を検出して記録
	if m.state != oldState {
		stateChange := fmt.Sprintf("%s -> %s", oldState.String(), m.state.String())
		m.debugStateChanges = append(m.debugStateChanges, stateChange)
		if len(m.debugStateChanges) > 10 {
			m.debugStateChanges = m.debugStateChanges[1:]
		}
		logger.Debug("STATE CHANGE: %s", stateChange)
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

	// デバッグ情報を表示（必要に応じて）
	if m.showDebugInfo {
		debugStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Background(lipgloss.Color("#000000")).
			Padding(1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#00FF00"))

		debugInfo := fmt.Sprintf("=== DEBUG INFO (%s to hide) ===\n", m.shortcutFormatter.formatKey("ctrl+d"))
		debugInfo += "Current State: " + m.state.String() + "\n"
		debugInfo += "Selected Index: " + fmt.Sprintf("%d", m.selectedIndex) + "\n"
		debugInfo += "Playlist Selected: " + fmt.Sprintf("%d", m.playlistSelectedIndex) + "\n"
		debugInfo += "Sections: " + fmt.Sprintf("%d", len(m.sections)) + "\n"
		debugInfo += "\nState Changes:\n"
		for _, change := range m.debugStateChanges {
			debugInfo += "  " + change + "\n"
		}
		debugInfo += "\nRecent Messages:\n"
		for i, msg := range m.debugMessageLog {
			debugInfo += fmt.Sprintf("  %d: %s\n", i, msg)
		}

		debugContent := debugStyle.Render(debugInfo)
		result = lipgloss.JoinHorizontal(lipgloss.Top, result, debugContent)
	}

	return result
}
