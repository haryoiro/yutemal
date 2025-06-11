package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// BatchUpdate collects multiple updates and applies them together.
type BatchUpdate struct {
	pendingUpdates []func(*Model)
	lastUpdate     time.Time
	minInterval    time.Duration
}

// NewBatchUpdate creates a new batch update manager.
func NewBatchUpdate() *BatchUpdate {
	return &BatchUpdate{
		pendingUpdates: make([]func(*Model), 0),
		minInterval:    16 * time.Millisecond, // ~60 FPS
	}
}

// Add adds an update to the batch.
func (bu *BatchUpdate) Add(update func(*Model)) {
	bu.pendingUpdates = append(bu.pendingUpdates, update)
}

// ShouldApply returns true if updates should be applied.
func (bu *BatchUpdate) ShouldApply() bool {
	return time.Since(bu.lastUpdate) >= bu.minInterval && len(bu.pendingUpdates) > 0
}

// Apply applies all pending updates.
func (bu *BatchUpdate) Apply(m *Model) {
	for _, update := range bu.pendingUpdates {
		update(m)
	}

	bu.pendingUpdates = bu.pendingUpdates[:0] // Clear without reallocating
	bu.lastUpdate = time.Now()
}

// Clear clears all pending updates.
func (bu *BatchUpdate) Clear() {
	bu.pendingUpdates = bu.pendingUpdates[:0]
}

// batchUpdateMsg is sent to trigger batch updates.
type batchUpdateMsg struct{}

// tickForBatchUpdate creates a command that ticks for batch updates.
func tickForBatchUpdate() tea.Cmd {
	return tea.Tick(16*time.Millisecond, func(t time.Time) tea.Msg {
		return batchUpdateMsg{}
	})
}
