package ui

import (
	"sync"
	"time"
)

// KeyDebouncer helps prevent key repeat flooding.
type KeyDebouncer struct {
	mu              sync.Mutex
	lastKeyTime     map[string]time.Time
	repeatDelay     time.Duration
	initialDelay    time.Duration
	consecutiveKeys map[string]int
}

// NewKeyDebouncer creates a new key debouncer.
func NewKeyDebouncer() *KeyDebouncer {
	return &KeyDebouncer{
		lastKeyTime:     make(map[string]time.Time),
		repeatDelay:     50 * time.Millisecond,  // Minimum time between repeated keys
		initialDelay:    300 * time.Millisecond, // Initial delay before fast repeat
		consecutiveKeys: make(map[string]int),
	}
}

// ShouldProcess returns true if the key event should be processed.
func (kd *KeyDebouncer) ShouldProcess(key string) bool {
	kd.mu.Lock()
	defer kd.mu.Unlock()

	now := time.Now()
	lastTime, exists := kd.lastKeyTime[key]

	if !exists {
		// First time pressing this key
		kd.lastKeyTime[key] = now
		kd.consecutiveKeys[key] = 1

		return true
	}

	timeSinceLastKey := now.Sub(lastTime)
	consecutive := kd.consecutiveKeys[key]

	// If enough time has passed, reset the counter
	if timeSinceLastKey > 500*time.Millisecond {
		kd.consecutiveKeys[key] = 1
		kd.lastKeyTime[key] = now

		return true
	}

	// Apply different delays based on consecutive key count
	var requiredDelay time.Duration
	if consecutive < 3 {
		// For first few presses, use initial delay
		requiredDelay = kd.initialDelay
	} else {
		// After that, allow faster repeat
		requiredDelay = kd.repeatDelay
	}

	if timeSinceLastKey >= requiredDelay {
		kd.consecutiveKeys[key]++
		kd.lastKeyTime[key] = now

		return true
	}

	// Too soon, skip this key press
	return false
}

// Reset clears the debouncer state for a specific key.
func (kd *KeyDebouncer) Reset(key string) {
	kd.mu.Lock()
	defer kd.mu.Unlock()
	delete(kd.lastKeyTime, key)
	delete(kd.consecutiveKeys, key)
}

// ResetAll clears all debouncer state.
func (kd *KeyDebouncer) ResetAll() {
	kd.mu.Lock()
	defer kd.mu.Unlock()
	kd.lastKeyTime = make(map[string]time.Time)
	kd.consecutiveKeys = make(map[string]int)
}

// getKeyString has been moved to key_filter.go
