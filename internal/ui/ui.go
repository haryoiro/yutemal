package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/haryoiro/yutemal/internal/logger"
	"github.com/haryoiro/yutemal/internal/structures"
	"github.com/haryoiro/yutemal/internal/systems"
)

// SetupRuneWidth configures the runewidth settings.
func SetupRuneWidth() {
	runewidth.DefaultCondition.EastAsianWidth = false
}

type ViewState int

const (
	PlaylistListView ViewState = iota
	PlaylistDetailView
	SearchView
)

func (v ViewState) String() string {
	switch v {
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

	// PlaylistListView fields
	playlists     []systems.Playlist
	selectedIndex int
	scrollOffset  int

	// PlaylistDetailView fields
	playlistTracks        []structures.Track
	playlistName          string
	playlistSelectedIndex int
	playlistScrollOffset  int

	// Queue display
	showQueue          bool
	queueWidth         int
	queueScrollOffset  int
	queueFocused       bool
	queueSelectedIndex int

	// Player focus
	playerFocused bool

	// Other fields
	playerState   structures.PlayerState
	searchQuery   string
	searchResults []structures.Track
	err           error
	marqueeOffset int
	marqueeTicker *time.Ticker
	lastUpdate    time.Time
	needsMarquee  bool // Track if marquee is needed for current content

	// Rainbow seekbar animation
	rainbowOffset int

	// Equalizer UI state
	eqPresetIndex int

	// Unified tick management
	tickActive bool

	// Mouse wheel throttling
	lastScrollTime time.Time
	scrollCooldown time.Duration

	// Key repeat prevention
	keyDebouncer    *KeyDebouncer
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
type errorMsg error

func RunSimple(systems *systems.Systems, config *structures.Config) error {
	m := Model{
		systems:           systems,
		config:            config,
		themeManager:      NewThemeManager(config.Theme),
		shortcutFormatter: NewShortcutFormatter(config),
		state:             PlaylistListView,
		playerHeight:      5,
		marqueeTicker:     time.NewTicker(500 * time.Millisecond),
		scrollCooldown:    20 * time.Millisecond,
		keyDebouncer:      NewKeyDebouncer(),
	}

	opts := []tea.ProgramOption{
		tea.WithMouseCellMotion(),
		tea.WithAltScreen(),
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
		m.loadPlaylists(),
		m.listenToPlayer(),
	)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	oldState := m.state

	msgType := ""
	switch msg := msg.(type) {
	case tea.KeyMsg:
		msgType = "KeyMsg: " + msg.String()
		logger.Debug("KeyMsg received: %v, current state: %v", msg, m.state)
	case playlistsLoadedMsg:
		msgType = "playlistsLoadedMsg"
		logger.Debug("playlistsLoadedMsg received, current state: %v", m.state)
	case tracksLoadedMsg:
		msgType = "tracksLoadedMsg"
		logger.Debug("tracksLoadedMsg received, current state: %v", m.state)
	case errorMsg:
		msgType = "errorMsg"
		logger.Debug("errorMsg received: %v, current state: %v", msg, m.state)
	case tickMsg:
	default:
		msgType = "other"
	}

	if msgType != "" && msgType != "other" {
		m.debugMessageLog = append(m.debugMessageLog, msgType+" @ "+m.state.String())
		if len(m.debugMessageLog) > 20 {
			m.debugMessageLog = m.debugMessageLog[1:]
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

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
		}

		if m.config.Theme.ProgressBarStyle == "rainbow" && m.playerState.TotalTime > 0 {
			m.rainbowOffset = (m.rainbowOffset + 1) % 360
		}

		if m.tickActive {
			return m, m.unifiedTickCmd()
		}

		return m, nil

	case playerUpdateMsg:
		m.playerState = structures.PlayerState(msg)
		if m.showQueue && !m.queueFocused && len(m.playerState.List) > 0 {
			visibleLines := max(m.contentHeight-4, 1)

			if m.playerState.Current < m.queueScrollOffset ||
				m.playerState.Current >= m.queueScrollOffset+visibleLines {
				m.queueScrollOffset = max(m.playerState.Current-visibleLines/2, 0)

				maxScrollOffset := max(len(m.playerState.List)-visibleLines, 0)

				if m.queueScrollOffset > maxScrollOffset {
					m.queueScrollOffset = maxScrollOffset
				}
			}
		}

		var cmds []tea.Cmd
		cmds = append(cmds, m.listenToPlayer())
		cmds = append(cmds, m.checkMarqueeCmd())

		if m.shouldTick() && !m.tickActive {
			m.tickActive = true
			cmds = append(cmds, m.unifiedTickCmd())
		}

		return m, tea.Batch(cmds...)

	case playlistsLoadedMsg:
		m.playlists = msg
		m.selectedIndex = 0
		m.scrollOffset = 0

		return m, nil

	case tracksLoadedMsg:
		if m.state == SearchView {
			m.searchResults = msg
			m.selectedIndex = 0
			m.scrollOffset = 0
			return m, nil
		}

		m.playlistTracks = msg

		if m.state == PlaylistDetailView {
			if m.playlistSelectedIndex >= len(msg) {
				m.playlistSelectedIndex = 0
			}

			if m.playlistScrollOffset > 0 && m.playlistScrollOffset >= len(msg) {
				m.playlistScrollOffset = 0
			}
		}

		return m, m.downloadAllSongs(msg)

	case errorMsg:
		m.err = msg
		return m, nil
	}

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

	borderColor := lipgloss.Color(m.config.Theme.Border)
	if m.themeManager != nil {
		borderColor = lipgloss.Color(m.config.Theme.Border)
	}

	mainStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, -3)

	playerBorderColor := borderColor
	if m.hasFocus("player") {
		playerBorderColor = lipgloss.Color(m.config.Theme.Selected)
	}

	playerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(playerBorderColor).
		Padding(0, 1)

	queueBorderColor := borderColor
	if m.hasFocus("queue") {
		queueBorderColor = lipgloss.Color(m.config.Theme.Selected)
	}

	queueStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(queueBorderColor).
		Padding(0, 1)

	mainV, mainH := mainStyle.GetFrameSize()
	playerV, playerH := playerStyle.GetFrameSize()
	queueV, queueH := queueStyle.GetFrameSize()

	if m.showQueue {
		m.queueWidth = min(max(m.width/3, 40), 80)
	} else {
		m.queueWidth = 0
	}

	mainContentWidth := m.width - mainH - m.queueWidth
	queueContentWidth := m.queueWidth - queueH
	playerContentWidth := m.width - playerH

	mainStyle = mainStyle.
		Width(mainContentWidth).
		Height(m.contentHeight)

	queueStyle = queueStyle.
		Width(queueContentWidth).
		Height(m.contentHeight)

	playerStyle = playerStyle.
		Width(playerContentWidth).
		Height(m.playerHeight - playerV)

	var content string

	switch m.state {
	case PlaylistListView:
		content = m.renderPlaylistList(mainContentWidth)
	case PlaylistDetailView:
		content = m.renderPlaylistDetail(mainContentWidth)
	case SearchView:
		content = m.renderSearch(mainContentWidth)
	}

	m.playerContentWidth = playerContentWidth
	player := m.renderPlayer()

	contentLines := strings.Split(content, "\n")
	maxContentLines := m.contentHeight - mainV + 2

	if len(contentLines) > maxContentLines {
		contentLines = contentLines[:maxContentLines]
	}

	content = strings.Join(contentLines, "\n")

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

	if m.showDebugInfo {
		debugStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Background(lipgloss.Color("#000000")).
			Padding(1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#00FF00"))

		var debugInfo strings.Builder
		debugInfo.WriteString(fmt.Sprintf("=== DEBUG INFO (%s to hide) ===\n", m.shortcutFormatter.formatKey("ctrl+d")))
		debugInfo.WriteString("Current State: " + m.state.String() + "\n")
		debugInfo.WriteString("Selected Index: " + fmt.Sprintf("%d", m.selectedIndex) + "\n")
		debugInfo.WriteString("Playlist Selected: " + fmt.Sprintf("%d", m.playlistSelectedIndex) + "\n")
		debugInfo.WriteString("Playlists: " + fmt.Sprintf("%d", len(m.playlists)) + "\n")
		debugInfo.WriteString("\nState Changes:\n")

		for _, change := range m.debugStateChanges {
			debugInfo.WriteString("  " + change + "\n")
		}

		debugInfo.WriteString("\nRecent Messages:\n")
		for i, msg := range m.debugMessageLog {
			debugInfo.WriteString(fmt.Sprintf("  %d: %s\n", i, msg))
		}

		debugContent := debugStyle.Render(debugInfo.String())
		result = lipgloss.JoinHorizontal(lipgloss.Top, result, debugContent)
	}

	return result
}

func (m *Model) shouldTick() bool {
	if m.needsMarquee {
		return true
	}
	if m.config.Theme.ProgressBarStyle == "rainbow" && m.playerState.TotalTime > 0 {
		return true
	}

	return false
}

func (m *Model) unifiedTickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		if !m.shouldTick() {
			m.tickActive = false
			return nil
		}

		return tickMsg(t)
	})
}
