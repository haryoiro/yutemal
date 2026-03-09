package queue

import (
	"math/rand"
	"testing"

	"github.com/haryoiro/yutemal/internal/structures"
)

func track(id string) structures.Track {
	return structures.Track{TrackID: id, Title: id}
}

func tracks(ids ...string) []structures.Track {
	t := make([]structures.Track, len(ids))
	for i, id := range ids {
		t[i] = track(id)
	}
	return t
}

func setup(ids ...string) *Queue {
	q := New()
	q.AddTracks(tracks(ids...))
	return q
}

// --- New / Empty ---

func TestNewIsEmpty(t *testing.T) {
	q := New()
	if q.Len() != 0 {
		t.Errorf("Len: got %d, want 0", q.Len())
	}
	if !q.IsEmpty() {
		t.Error("IsEmpty: got false, want true")
	}
	if q.ValidCurrent() {
		t.Error("ValidCurrent: got true on empty queue")
	}
	if _, ok := q.CurrentTrack(); ok {
		t.Error("CurrentTrack: got ok on empty queue")
	}
}

// --- AddTracks ---

func TestAddTracks(t *testing.T) {
	q := setup("a", "b", "c")

	if q.Len() != 3 {
		t.Fatalf("Len: got %d, want 3", q.Len())
	}
	cur, ok := q.CurrentTrack()
	if !ok || cur.TrackID != "a" {
		t.Errorf("CurrentTrack: got %s, want a", cur.TrackID)
	}
}

// --- Next / Previous ---

func TestNext(t *testing.T) {
	q := setup("a", "b", "c")

	if !q.Next() {
		t.Error("Next from 0: got false")
	}
	if q.Current != 1 {
		t.Errorf("Current: got %d, want 1", q.Current)
	}

	if !q.Next() {
		t.Error("Next from 1: got false")
	}
	if q.Current != 2 {
		t.Errorf("Current: got %d, want 2", q.Current)
	}

	if q.Next() {
		t.Error("Next from last: got true")
	}
	if q.Current != 2 {
		t.Errorf("Current after failed Next: got %d, want 2", q.Current)
	}
}

func TestNextOnEmpty(t *testing.T) {
	q := New()
	if q.Next() {
		t.Error("Next on empty: got true")
	}
}

func TestPrevious(t *testing.T) {
	q := setup("a", "b", "c")
	q.Current = 2

	if !q.Previous() {
		t.Error("Previous from 2: got false")
	}
	if q.Current != 1 {
		t.Errorf("Current: got %d, want 1", q.Current)
	}

	if !q.Previous() {
		t.Error("Previous from 1: got false")
	}
	if q.Current != 0 {
		t.Errorf("Current: got %d, want 0", q.Current)
	}

	if q.Previous() {
		t.Error("Previous from 0: got true")
	}
	if q.Current != 0 {
		t.Errorf("Current after failed Previous: got %d, want 0", q.Current)
	}
}

// --- JumpTo ---

func TestJumpTo(t *testing.T) {
	q := setup("a", "b", "c")

	if !q.JumpTo(2) {
		t.Error("JumpTo(2): got false")
	}
	if q.Current != 2 {
		t.Errorf("Current: got %d, want 2", q.Current)
	}

	if q.JumpTo(-1) {
		t.Error("JumpTo(-1): got true")
	}
	if q.JumpTo(3) {
		t.Error("JumpTo(3): got true")
	}
	if q.Current != 2 {
		t.Errorf("Current unchanged: got %d, want 2", q.Current)
	}
}

func TestJumpToOnEmpty(t *testing.T) {
	q := New()
	if q.JumpTo(0) {
		t.Error("JumpTo(0) on empty: got true")
	}
}

// --- InsertAfterCurrent ---

func TestInsertAfterCurrentEmpty(t *testing.T) {
	q := New()
	q.InsertAfterCurrent(track("x"))

	if q.Len() != 1 {
		t.Fatalf("Len: got %d, want 1", q.Len())
	}
	if q.Current != 0 {
		t.Errorf("Current: got %d, want 0", q.Current)
	}
	if q.Tracks[0].TrackID != "x" {
		t.Errorf("Track: got %s, want x", q.Tracks[0].TrackID)
	}
}

func TestInsertAfterCurrentMiddle(t *testing.T) {
	q := setup("a", "b", "c")
	q.Current = 1 // at "b"
	q.InsertAfterCurrent(track("x"))

	want := []string{"a", "b", "x", "c"}
	if q.Len() != len(want) {
		t.Fatalf("Len: got %d, want %d", q.Len(), len(want))
	}
	for i, id := range want {
		if q.Tracks[i].TrackID != id {
			t.Errorf("Tracks[%d]: got %s, want %s", i, q.Tracks[i].TrackID, id)
		}
	}
	if q.Current != 1 {
		t.Errorf("Current unchanged: got %d, want 1", q.Current)
	}
}

func TestInsertAfterCurrentEnd(t *testing.T) {
	q := setup("a", "b")
	q.Current = 1 // at "b" (last)
	q.InsertAfterCurrent(track("x"))

	want := []string{"a", "b", "x"}
	if q.Len() != len(want) {
		t.Fatalf("Len: got %d, want %d", q.Len(), len(want))
	}
	for i, id := range want {
		if q.Tracks[i].TrackID != id {
			t.Errorf("Tracks[%d]: got %s, want %s", i, q.Tracks[i].TrackID, id)
		}
	}
}

// --- DeleteAt ---

func TestDeleteAtBeforeCurrent(t *testing.T) {
	q := setup("a", "b", "c", "d")
	q.Current = 2 // at "c"

	deleted := q.DeleteAt(0) // delete "a"
	if deleted {
		t.Error("deletedCurrent: got true, want false")
	}
	if q.Current != 1 {
		t.Errorf("Current: got %d, want 1 (shifted back)", q.Current)
	}
	if q.Len() != 3 {
		t.Errorf("Len: got %d, want 3", q.Len())
	}
	cur, _ := q.CurrentTrack()
	if cur.TrackID != "c" {
		t.Errorf("CurrentTrack: got %s, want c", cur.TrackID)
	}
}

func TestDeleteAtCurrent(t *testing.T) {
	q := setup("a", "b", "c")
	q.Current = 1 // at "b"

	deleted := q.DeleteAt(1)
	if !deleted {
		t.Error("deletedCurrent: got false, want true")
	}
	if q.Current != 1 {
		t.Errorf("Current: got %d, want 1", q.Current)
	}
	cur, _ := q.CurrentTrack()
	if cur.TrackID != "c" {
		t.Errorf("CurrentTrack: got %s, want c", cur.TrackID)
	}
}

func TestDeleteAtCurrentLast(t *testing.T) {
	q := setup("a", "b", "c")
	q.Current = 2 // at "c" (last)

	deleted := q.DeleteAt(2)
	if !deleted {
		t.Error("deletedCurrent: got false, want true")
	}
	if q.Current != 1 {
		t.Errorf("Current: got %d, want 1 (wrapped back)", q.Current)
	}
	cur, _ := q.CurrentTrack()
	if cur.TrackID != "b" {
		t.Errorf("CurrentTrack: got %s, want b", cur.TrackID)
	}
}

func TestDeleteAtAfterCurrent(t *testing.T) {
	q := setup("a", "b", "c")
	q.Current = 0

	deleted := q.DeleteAt(2) // delete "c"
	if deleted {
		t.Error("deletedCurrent: got true")
	}
	if q.Current != 0 {
		t.Errorf("Current: got %d, want 0 (unchanged)", q.Current)
	}
}

func TestDeleteAtOutOfBounds(t *testing.T) {
	q := setup("a", "b")
	deleted := q.DeleteAt(-1)
	if deleted {
		t.Error("DeleteAt(-1): got true")
	}
	deleted = q.DeleteAt(5)
	if deleted {
		t.Error("DeleteAt(5): got true")
	}
	if q.Len() != 2 {
		t.Errorf("Len unchanged: got %d, want 2", q.Len())
	}
}

func TestDeleteAtOnlyElement(t *testing.T) {
	q := setup("a")
	deleted := q.DeleteAt(0)
	if !deleted {
		t.Error("deletedCurrent: got false")
	}
	if q.Len() != 0 {
		t.Errorf("Len: got %d, want 0", q.Len())
	}
	if q.Current != 0 {
		t.Errorf("Current: got %d, want 0", q.Current)
	}
}

// --- DeleteCurrent ---

func TestDeleteCurrent(t *testing.T) {
	q := setup("a", "b", "c")
	q.Current = 1
	q.DeleteCurrent()

	if q.Len() != 2 {
		t.Fatalf("Len: got %d, want 2", q.Len())
	}
	if q.Current != 1 {
		t.Errorf("Current: got %d, want 1", q.Current)
	}
	cur, _ := q.CurrentTrack()
	if cur.TrackID != "c" {
		t.Errorf("CurrentTrack: got %s, want c", cur.TrackID)
	}
}

func TestDeleteCurrentLast(t *testing.T) {
	q := setup("a", "b")
	q.Current = 1
	q.DeleteCurrent()

	if q.Current != 0 {
		t.Errorf("Current: got %d, want 0", q.Current)
	}
}

func TestDeleteCurrentOnEmpty(t *testing.T) {
	q := New()
	q.DeleteCurrent() // should not panic
	if q.Len() != 0 {
		t.Error("Len should stay 0")
	}
}

// --- Shuffle ---

func TestShuffleDeterministic(t *testing.T) {
	q := setup("a", "b", "c", "d", "e")
	q.Current = 1 // at "b"

	rng := rand.New(rand.NewSource(42))
	q.Shuffle(rng)

	// Current track should not move
	if q.Tracks[0].TrackID != "a" {
		t.Error("Track before current should not move")
	}
	if q.Tracks[1].TrackID != "b" {
		t.Error("Current track should not move")
	}
	if q.Current != 1 {
		t.Error("Current index should not change")
	}

	// Remaining tracks should be shuffled (deterministic with seed 42)
	// Just verify they contain the same elements
	remaining := make(map[string]bool)
	for _, tr := range q.Tracks[2:] {
		remaining[tr.TrackID] = true
	}
	for _, id := range []string{"c", "d", "e"} {
		if !remaining[id] {
			t.Errorf("Missing track %s after shuffle", id)
		}
	}
}

func TestShuffleNoTracksAfterCurrent(t *testing.T) {
	q := setup("a", "b")
	q.Current = 1 // at last

	rng := rand.New(rand.NewSource(42))
	q.Shuffle(rng) // should not panic

	if q.Len() != 2 {
		t.Error("Length should not change")
	}
}

func TestShuffleEmpty(t *testing.T) {
	q := New()
	rng := rand.New(rand.NewSource(42))
	q.Shuffle(rng) // should not panic
}

// --- ReplaceAfterCurrent ---

func TestReplaceAfterCurrent(t *testing.T) {
	q := setup("a", "b", "c")
	q.Current = 0

	advanced := q.ReplaceAfterCurrent(tracks("x", "y"))
	if !advanced {
		t.Error("ReplaceAfterCurrent should advance to next")
	}
	if q.Current != 1 {
		t.Errorf("Current: got %d, want 1", q.Current)
	}

	want := []string{"a", "x", "y"}
	if q.Len() != len(want) {
		t.Fatalf("Len: got %d, want %d", q.Len(), len(want))
	}
	for i, id := range want {
		if q.Tracks[i].TrackID != id {
			t.Errorf("Tracks[%d]: got %s, want %s", i, q.Tracks[i].TrackID, id)
		}
	}
}

func TestReplaceAfterCurrentAtEnd(t *testing.T) {
	q := setup("a")
	q.Current = 0

	advanced := q.ReplaceAfterCurrent(tracks("x"))
	if !advanced {
		t.Error("should advance")
	}
	if q.Current != 1 {
		t.Errorf("Current: got %d, want 1", q.Current)
	}
}

func TestReplaceAfterCurrentEmpty(t *testing.T) {
	q := New()
	advanced := q.ReplaceAfterCurrent(tracks("x"))
	// Current is 0, nothing before it, so next should advance from 0 to... well, tracks are appended after 0
	// But Tracks[0:0+1] = Tracks[0:1] which is empty queue (no 0th element)
	// Actually: Current = 0, Tracks is empty.
	// q.Current+1 < len(q.Tracks) is false since 0+1=1 >= 0
	// So no truncation. Then append "x". Tracks = ["x"]. Then Next() = q.Current+1 >= 1, false.
	if advanced {
		t.Error("should not advance on empty base")
	}
}

// --- Clear ---

func TestClear(t *testing.T) {
	q := setup("a", "b", "c")
	q.Current = 2
	q.Clear()

	if !q.IsEmpty() {
		t.Error("should be empty")
	}
	if q.Current != 0 {
		t.Error("Current should be 0")
	}
}

// --- Edge cases ---

func TestDeleteAllOneByOne(t *testing.T) {
	q := setup("a", "b", "c")
	q.DeleteAt(0)
	q.DeleteAt(0)
	q.DeleteAt(0)

	if !q.IsEmpty() {
		t.Errorf("should be empty, got %d tracks", q.Len())
	}
}

func TestInsertThenDelete(t *testing.T) {
	q := setup("a", "c")
	q.Current = 0
	q.InsertAfterCurrent(track("b"))

	// Should be [a, b, c], current at 0
	if q.Tracks[1].TrackID != "b" {
		t.Errorf("inserted track: got %s, want b", q.Tracks[1].TrackID)
	}

	q.DeleteAt(1) // delete "b"
	want := []string{"a", "c"}
	for i, id := range want {
		if q.Tracks[i].TrackID != id {
			t.Errorf("Tracks[%d]: got %s, want %s", i, q.Tracks[i].TrackID, id)
		}
	}
}
