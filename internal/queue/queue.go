package queue

import (
	"math/rand"

	"github.com/haryoiro/yutemal/internal/structures"
)

// Queue manages an ordered list of tracks with a current position pointer.
// All methods are pure state manipulation with no I/O side effects.
type Queue struct {
	Tracks  []structures.Track
	Current int
}

// New creates an empty queue.
func New() *Queue {
	return &Queue{}
}

// Len returns the number of tracks.
func (q *Queue) Len() int {
	return len(q.Tracks)
}

// IsEmpty returns whether the queue has no tracks.
func (q *Queue) IsEmpty() bool {
	return len(q.Tracks) == 0
}

// CurrentTrack returns the current track. Returns false if invalid.
func (q *Queue) CurrentTrack() (structures.Track, bool) {
	if !q.ValidCurrent() {
		return structures.Track{}, false
	}
	return q.Tracks[q.Current], true
}

// ValidCurrent returns whether the current index is valid.
func (q *Queue) ValidCurrent() bool {
	return q.Current >= 0 && q.Current < len(q.Tracks)
}

// Next advances to the next track. Returns true if advanced.
func (q *Queue) Next() bool {
	if q.Current+1 >= len(q.Tracks) {
		return false
	}
	q.Current++
	return true
}

// Previous goes back to the previous track. Returns true if moved.
func (q *Queue) Previous() bool {
	if q.Current <= 0 {
		return false
	}
	q.Current--
	return true
}

// JumpTo sets the current index. Returns false if out of bounds.
func (q *Queue) JumpTo(index int) bool {
	if index < 0 || index >= len(q.Tracks) {
		return false
	}
	q.Current = index
	return true
}

// AddTracks appends tracks to the end.
func (q *Queue) AddTracks(tracks []structures.Track) {
	q.Tracks = append(q.Tracks, tracks...)
}

// InsertAfterCurrent inserts a track after the current position.
// If queue is empty, the track becomes the first and current.
func (q *Queue) InsertAfterCurrent(track structures.Track) {
	if len(q.Tracks) == 0 {
		q.Tracks = append(q.Tracks, track)
		q.Current = 0
		return
	}
	insertPos := min(q.Current+1, len(q.Tracks))
	q.Tracks = append(q.Tracks[:insertPos],
		append([]structures.Track{track}, q.Tracks[insertPos:]...)...)
}

// DeleteAt removes a track at the given index.
// Returns whether the deleted track was the current one.
func (q *Queue) DeleteAt(index int) (deletedCurrent bool) {
	if index < 0 || index >= len(q.Tracks) {
		return false
	}
	deletedCurrent = index == q.Current
	q.Tracks = append(q.Tracks[:index], q.Tracks[index+1:]...)

	if index < q.Current {
		q.Current--
	} else if deletedCurrent {
		if q.Current >= len(q.Tracks) && q.Current > 0 {
			q.Current--
		}
	}
	return deletedCurrent
}

// DeleteCurrent removes the current track.
func (q *Queue) DeleteCurrent() {
	if !q.ValidCurrent() {
		return
	}
	q.Tracks = append(q.Tracks[:q.Current], q.Tracks[q.Current+1:]...)
	if q.Current >= len(q.Tracks) && q.Current > 0 {
		q.Current--
	}
}

// Shuffle shuffles all tracks after the current position.
// Uses the provided random source for deterministic testing.
func (q *Queue) Shuffle(rng *rand.Rand) {
	if len(q.Tracks) <= q.Current+1 {
		return
	}
	remaining := q.Tracks[q.Current+1:]
	for i := len(remaining) - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		remaining[i], remaining[j] = remaining[j], remaining[i]
	}
}

// ReplaceAfterCurrent removes all tracks after current and appends new ones.
// Advances to the next track. Returns true if there was a next track.
func (q *Queue) ReplaceAfterCurrent(tracks []structures.Track) bool {
	if q.Current+1 < len(q.Tracks) {
		q.Tracks = q.Tracks[:q.Current+1]
	}
	q.Tracks = append(q.Tracks, tracks...)
	return q.Next()
}

// Clear removes all tracks and resets the index.
func (q *Queue) Clear() {
	q.Tracks = nil
	q.Current = 0
}
