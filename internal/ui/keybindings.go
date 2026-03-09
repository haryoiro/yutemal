package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/haryoiro/yutemal/internal/logger"
	"github.com/haryoiro/yutemal/internal/structures"
)

// isKey checks if the pressed key matches the configured keybinding.
func (m *Model) isKey(msg tea.KeyMsg, key string) bool {
	if key == "" {
		return false
	}

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
		return msg.Type == tea.KeyRunes && msg.String() == key
	}
}

// isKeyInList checks if the pressed key matches any of the configured keybindings.
func (m *Model) isKeyInList(msg tea.KeyMsg, bindings []string) bool {
	key := msg.String()

	if len(key) > 1 && (key[0] == '[' || key[0] == 27) {
		logger.Debug("Ignoring potential mouse escape sequence: %s", key)
		return false
	}

	for _, binding := range bindings {
		if binding == "backspace" {
			return msg.Type == tea.KeyBackspace
		}

		if key == binding {
			return true
		}
	}

	return false
}

// handleKeyPress processes keyboard input and delegates to appropriate handlers.
func (m *Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	kb := m.config.KeyBindings

	logger.Debug("Raw key event: type=%d, string=%s, alt=%t, runes=%v, focus=%d",
		msg.Type, msg.String(), msg.Alt, msg.Runes, m.getFocusedPane())

	// Global quit (always works)
	if m.isKey(msg, kb.Quit) {
		return m, tea.Quit
	}

	// Tab cycles focus: Main → Queue (if visible) → Player → Main
	if m.isKey(msg, "tab") {
		m.cycleFocus()
		return m, nil
	}

	keyStr := getKeyString(msg)

	if !m.shouldProcessKey(keyStr, msg) {
		return m, nil
	}

	// Dispatch based on current focus
	switch m.getFocusedPane() {
	case FocusPlayer:
		return m.handlePlayerFocusKeys(msg)
	case FocusQueue:
		return m.handleQueueFocusKeys(msg)
	case FocusSearch:
		return m.handleSearchFocusKeys(msg)
	default:
		return m.handleMainFocusKeys(msg)
	}
}

// handlePlayerFocusKeys handles keys when the player pane is focused.
func (m *Model) handlePlayerFocusKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	kb := m.config.KeyBindings

	// ↑↓ = volume
	if m.isKeyInList(msg, kb.MoveUp) || m.isKeyInList(msg, kb.VolumeUp) {
		return m.volumeUp()
	}

	if m.isKeyInList(msg, kb.MoveDown) || m.isKeyInList(msg, kb.VolumeDown) {
		return m.volumeDown()
	}

	// ←→ = seek
	if m.isKey(msg, kb.SeekForward) {
		return m.seekForward()
	}

	if m.isKey(msg, kb.SeekBackward) {
		return m.seekBackward()
	}

	// space = play/pause
	if m.isKey(msg, kb.PlayPause) {
		return m.togglePlayPause()
	}

	// e = EQ cycle
	if m.isKey(msg, kb.ToggleEQ) {
		return m.eqCyclePreset()
	}

	// s = shuffle
	if m.isKey(msg, kb.Shuffle) {
		return m.shuffleQueue()
	}

	// q = toggle queue
	if m.isKey(msg, "q") {
		return m.toggleQueue()
	}

	// Back = unfocus player (return to main)
	if m.isKeyInList(msg, kb.Back) {
		m.setFocus(FocusMain)
		return m, nil
	}

	return m, nil
}

// handleQueueFocusKeys handles keys when the queue pane is focused.
func (m *Model) handleQueueFocusKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	kb := m.config.KeyBindings

	// Navigation
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

	// Select = play from here
	if m.isKeyInList(msg, kb.Select) {
		return m.handleQueueSelection()
	}

	// d = remove track
	if m.isKey(msg, kb.RemoveTrack) {
		return m.removeTrack()
	}

	// q = hide queue
	if m.isKey(msg, "q") {
		return m.toggleQueue()
	}

	// Back = unfocus queue (return to main)
	if m.isKeyInList(msg, kb.Back) {
		m.setFocus(FocusMain)
		return m, nil
	}

	// Player controls (always available)
	if m.isKey(msg, kb.PlayPause) {
		return m.togglePlayPause()
	}

	if m.isKey(msg, kb.SeekForward) {
		return m.seekForward()
	}

	if m.isKey(msg, kb.SeekBackward) {
		return m.seekBackward()
	}

	return m, nil
}

// handleSearchFocusKeys handles keys when in search view.
func (m *Model) handleSearchFocusKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	kb := m.config.KeyBindings

	// Back = exit search
	if m.isKeyInList(msg, kb.Back) {
		now := time.Now()
		if m.lastBackKeyTime != nil && now.Sub(*m.lastBackKeyTime) < 500*time.Millisecond {
			return m, nil
		}

		m.lastBackKeyTime = &now
		return m.navigateBack()
	}

	// If search results exist and not typing, allow navigation
	if len(m.searchResults) > 0 {
		if m.isKeyInList(msg, kb.MoveUp) {
			return m.moveUp()
		}

		if m.isKeyInList(msg, kb.MoveDown) {
			return m.moveDown()
		}
	}

	// Text input handling
	return m.handleSearchKeys(msg)
}

// handleMainFocusKeys handles keys when the main pane is focused.
func (m *Model) handleMainFocusKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	kb := m.config.KeyBindings

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

	// Player controls (available from main too)
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

	// Equalizer
	if m.isKey(msg, kb.ToggleEQ) {
		return m.eqCyclePreset()
	}

	// Queue toggle
	if m.isKey(msg, "q") {
		return m.toggleQueue()
	}

	// Shuffle
	if m.isKey(msg, kb.Shuffle) {
		return m.shuffleQueue()
	}

	// Remove track
	if m.isKey(msg, kb.RemoveTrack) {
		return m.removeTrack()
	}

	// Selection/Enter
	if m.isKeyInList(msg, kb.Select) {
		return m.handleQueueSelection()
	}

	// Back navigation
	if m.isKeyInList(msg, kb.Back) {
		now := time.Now()
		if m.lastBackKeyTime != nil && now.Sub(*m.lastBackKeyTime) < 500*time.Millisecond {
			return m, nil
		}

		m.lastBackKeyTime = &now
		return m.navigateBack()
	}

	// View-specific keys
	switch m.state {
	case PlaylistListView:
		if m.isKeyInList(msg, kb.Search) {
			return m.startSearch()
		}
	case PlaylistDetailView:
		return m.handlePlaylistDetailKeys(msg)
	}

	return m, nil
}

// handleSearchKeys handles text input in the search view.
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

// handlePlaylistDetailKeys handles keys specific to the playlist detail view.
func (m *Model) handlePlaylistDetailKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.isKey(msg, "a") {
		if len(m.playlistTracks) > 0 && m.playlistSelectedIndex < len(m.playlistTracks) {
			track := m.playlistTracks[m.playlistSelectedIndex]
			m.systems.Player.SendAction(structures.InsertTrackAfterCurrentAction{Track: track})
		}
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
