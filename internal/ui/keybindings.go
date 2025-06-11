package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/haryoiro/yutemal/internal/logger"
	"github.com/haryoiro/yutemal/internal/structures"
)

// キーバインド関連のヘルパー関数

// isKey checks if the pressed key matches the configured keybinding
func (m *Model) isKey(msg tea.KeyMsg, key string) bool {
	if key == "" {
		return false
	}

	// Handle special keys
	switch key {
	case "ctrl+c":
		return msg.Type == tea.KeyCtrlC
	case "ctrl+d":
		return msg.Type == tea.KeyCtrlD
	case "space":
		return msg.Type == tea.KeySpace
	case "enter":
		return msg.Type == tea.KeyEnter
	case "esc":
		// Only accept actual ESC key type, not string matches
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
	case "pgup":
		return msg.Type == tea.KeyPgUp
	case "pgdown":
		return msg.Type == tea.KeyPgDown
	default:
		// Handle regular character keys
		return msg.Type == tea.KeyRunes && msg.String() == key
	}
}

// isKeyInList checks if the pressed key matches any of the configured keybindings
func (m *Model) isKeyInList(msg tea.KeyMsg, bindings []string) bool {
	key := msg.String()
	
	// Filter out mouse event escape sequences that might be mistaken for keys
	if len(key) > 1 && (key[0] == '[' || key[0] == 27) {
		logger.Debug("Ignoring potential mouse escape sequence: %s", key)
		return false
	}
	
	for _, binding := range bindings {
		// For backspace key, only accept actual backspace key type
		if binding == "backspace" {
			return msg.Type == tea.KeyBackspace
		}
		if key == binding {
			return true
		}
	}
	return false
}


// handleKeyPress processes keyboard input and delegates to appropriate handlers
func (m *Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	kb := m.config.KeyBindings
	
	// Log all key events for debugging
	logger.Debug("Raw key event: type=%d, string=%s, alt=%t, runes=%v", 
		msg.Type, msg.String(), msg.Alt, msg.Runes)

	// Global quit command (always process without debouncing)
	if m.isKey(msg, kb.Quit) {
		return m, tea.Quit
	}

	// Get key string for debouncing
	keyStr := getKeyString(msg)

	// Check if this key press should be processed
	if !m.shouldProcessKey(keyStr, msg) {
		return m, nil
	}

	// Navigation keys
	if m.isKeyInList(msg, kb.MoveUp) {
		return m.moveUp()
	}
	if m.isKeyInList(msg, kb.MoveDown) {
		return m.moveDown()
	}

	// Page navigation
	switch msg.String() {
	case "g":
		return m.jumpToTop()
	case "G":
		return m.jumpToBottom()
	case "ctrl+b", "pgup":
		return m.pageUp()
	case "ctrl+f", "pgdown":
		return m.pageDown()
	}

	// Player controls
	if m.isKey(msg, kb.PlayPause) {
		return m.togglePlayPause()
	}
	if m.isKeyInList(msg, kb.VolumeUp) {
		return m.volumeUp()
	}
	if m.isKeyInList(msg, kb.VolumeDown) {
		return m.volumeDown()
	}
	if m.isKey(msg, kb.SeekForward) {
		return m.seekForward()
	}
	if m.isKey(msg, kb.SeekBackward) {
		return m.seekBackward()
	}

	// Queue and playlist controls
	if m.isKey(msg, kb.Shuffle) {
		return m.shuffleQueue()
	}
	if m.isKey(msg, kb.RemoveTrack) {
		return m.removeTrack()
	}
	if m.isKey(msg, "q") {
		return m.toggleQueue()
	}
	if m.isKey(msg, "tab") {
		return m.toggleQueueFocus()
	}
	// Additional quit key for compatibility (Ctrl+D)
	if msg.Type == tea.KeyCtrlD {
		return m, tea.Quit
	}

	// Selection/Enter
	if m.isKeyInList(msg, kb.Select) {
		return m.handleQueueSelection()
	}

	// Back navigation
	// Skip backspace as Back key in SearchView (it's used for text deletion)
	if m.isKeyInList(msg, kb.Back) {
		if m.state == SearchView && msg.String() == "backspace" {
			// Let SearchView handle backspace for text deletion
			logger.Debug("Skipping backspace as Back key in SearchView")
			return m, nil
		}
		
		// Add strict debouncing for back keys to prevent rapid double-presses
		now := time.Now()
		if m.lastBackKeyTime != nil && now.Sub(*m.lastBackKeyTime) < 500*time.Millisecond {
			logger.Debug("Back key debounced: %s (last processed %.0fms ago)", 
				msg.String(), now.Sub(*m.lastBackKeyTime).Seconds()*1000)
			return m, nil
		}
		m.lastBackKeyTime = &now
		
		logger.Debug("Back key pressed: %s in state %s with focus %d", msg.String(), m.state, m.getFocusedPane())
		return m.navigateBack()
	}

	// View-specific keys
	switch m.state {
	case HomeView:
		return m.handleHomeKeys(msg)
	case SearchView:
		return m.handleSearchKeys(msg)
	case PlaylistDetailView:
		return m.handlePlaylistDetailKeys(msg)
	}

	// Home key
	if m.isKey(msg, kb.Home) {
		return m.navigateHome()
	}

	// Search key
	if m.isKey(msg, kb.Search) && m.state == HomeView {
		return m.startSearch()
	}

	return m, nil
}

// handleHomeKeys handles keys specific to the home view
func (m *Model) handleHomeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Currently no home-specific keys since section navigation conflicts with player controls
	// Tab is used for queue focus toggle
	// Left/Right are used for seeking
	return m, nil
}

// handleSearchKeys handles keys specific to the search view
func (m *Model) handleSearchKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if strings.TrimSpace(m.searchQuery) != "" {
			return m, m.performSearch()
		}
	case "backspace":
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
		}
	default:
		if msg.Type == tea.KeyRunes {
			m.searchQuery += string(msg.Runes)
		}
	}
	return m, nil
}

// handlePlaylistDetailKeys handles keys specific to the playlist detail view
func (m *Model) handlePlaylistDetailKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Add single track to queue after current (use 'a' key)
	if m.isKey(msg, "a") {
		if len(m.playlistTracks) > 0 && m.playlistSelectedIndex < len(m.playlistTracks) {
			track := m.playlistTracks[m.playlistSelectedIndex]
			m.systems.Player.SendAction(structures.InsertTrackAfterCurrentAction{Track: track})
		}
	}
	return m, nil
}

// Helper navigation methods
func (m *Model) nextSection() (tea.Model, tea.Cmd) {
	if m.currentSectionIndex < len(m.sections)-1 {
		m.currentSectionIndex++
		m.selectedIndex = 0
		m.scrollOffset = 0
	}
	return m, nil
}

func (m *Model) prevSection() (tea.Model, tea.Cmd) {
	if m.currentSectionIndex > 0 {
		m.currentSectionIndex--
		m.selectedIndex = 0
		m.scrollOffset = 0
	}
	return m, nil
}

func (m *Model) navigateHome() (tea.Model, tea.Cmd) {
	if m.state == PlaylistDetailView {
		m.state = HomeView
		m.selectedIndex = 0
		m.scrollOffset = 0
	}
	return m, nil
}

func (m *Model) startSearch() (tea.Model, tea.Cmd) {
	m.state = SearchView
	m.searchQuery = ""
	m.searchResults = nil
	m.selectedIndex = 0
	m.scrollOffset = 0
	return m, nil
}
