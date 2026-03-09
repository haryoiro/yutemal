package listnav

import "testing"

// --- New ---

func TestNew(t *testing.T) {
	ln := New(10, 5)
	if ln.ListSize != 10 || ln.PageSize != 5 {
		t.Errorf("New(10, 5): got ListSize=%d, PageSize=%d", ln.ListSize, ln.PageSize)
	}
	if ln.Selected != 0 || ln.ScrollOffset != 0 {
		t.Error("New should start at 0")
	}
}

// --- MoveUp / MoveDown ---

func TestMoveDown(t *testing.T) {
	ln := New(5, 3)

	if !ln.MoveDown() {
		t.Error("MoveDown from 0: got false")
	}
	if ln.Selected != 1 {
		t.Errorf("Selected: got %d, want 1", ln.Selected)
	}
}

func TestMoveDownAtEnd(t *testing.T) {
	ln := New(3, 3)
	ln.Selected = 2

	if ln.MoveDown() {
		t.Error("MoveDown at end: got true")
	}
	if ln.Selected != 2 {
		t.Errorf("Selected unchanged: got %d, want 2", ln.Selected)
	}
}

func TestMoveUp(t *testing.T) {
	ln := New(5, 3)
	ln.Selected = 2

	if !ln.MoveUp() {
		t.Error("MoveUp from 2: got false")
	}
	if ln.Selected != 1 {
		t.Errorf("Selected: got %d, want 1", ln.Selected)
	}
}

func TestMoveUpAtTop(t *testing.T) {
	ln := New(5, 3)

	if ln.MoveUp() {
		t.Error("MoveUp at top: got true")
	}
	if ln.Selected != 0 {
		t.Errorf("Selected unchanged: got %d, want 0", ln.Selected)
	}
}

func TestMoveDownEmptyList(t *testing.T) {
	ln := New(0, 3)
	if ln.MoveDown() {
		t.Error("MoveDown on empty: got true")
	}
}

func TestMoveDownSingleItem(t *testing.T) {
	ln := New(1, 3)
	if ln.MoveDown() {
		t.Error("MoveDown on single item: got true")
	}
}

// --- PageUp / PageDown ---

func TestPageDown(t *testing.T) {
	ln := New(20, 5)
	ln.PageDown()

	if ln.Selected != 5 {
		t.Errorf("Selected after PageDown: got %d, want 5", ln.Selected)
	}
}

func TestPageDownNearEnd(t *testing.T) {
	ln := New(8, 5)
	ln.Selected = 5
	ln.PageDown()

	if ln.Selected != 7 {
		t.Errorf("Selected: got %d, want 7 (clamped to last)", ln.Selected)
	}
}

func TestPageUp(t *testing.T) {
	ln := New(20, 5)
	ln.Selected = 10
	ln.ScrollOffset = 8
	ln.PageUp()

	if ln.Selected != 5 {
		t.Errorf("Selected after PageUp: got %d, want 5", ln.Selected)
	}
}

func TestPageUpNearTop(t *testing.T) {
	ln := New(20, 5)
	ln.Selected = 2
	ln.PageUp()

	if ln.Selected != 0 {
		t.Errorf("Selected: got %d, want 0 (clamped to top)", ln.Selected)
	}
}

func TestPageDownEmptyList(t *testing.T) {
	ln := New(0, 5)
	ln.PageDown()
	if ln.Selected != 0 {
		t.Errorf("Selected: got %d, want 0", ln.Selected)
	}
}

// --- JumpToTop / JumpToBottom ---

func TestJumpToTop(t *testing.T) {
	ln := New(10, 5)
	ln.Selected = 7
	ln.ScrollOffset = 5
	ln.JumpToTop()

	if ln.Selected != 0 {
		t.Errorf("Selected: got %d, want 0", ln.Selected)
	}
	if ln.ScrollOffset != 0 {
		t.Errorf("ScrollOffset: got %d, want 0", ln.ScrollOffset)
	}
}

func TestJumpToBottom(t *testing.T) {
	ln := New(10, 5)
	ln.JumpToBottom()

	if ln.Selected != 9 {
		t.Errorf("Selected: got %d, want 9", ln.Selected)
	}
}

func TestJumpToBottomEmpty(t *testing.T) {
	ln := New(0, 5)
	ln.JumpToBottom()

	if ln.Selected != 0 {
		t.Errorf("Selected: got %d, want 0", ln.Selected)
	}
}

// --- AdjustScroll ---

func TestAdjustScrollSelectedAboveViewport(t *testing.T) {
	ln := &ListNav{Selected: 2, ScrollOffset: 5, ListSize: 20, PageSize: 5}
	ln.AdjustScroll()

	if ln.ScrollOffset != 2 {
		t.Errorf("ScrollOffset: got %d, want 2 (snapped to selected)", ln.ScrollOffset)
	}
}

func TestAdjustScrollSelectedBelowViewport(t *testing.T) {
	ln := &ListNav{Selected: 10, ScrollOffset: 0, ListSize: 20, PageSize: 5}
	ln.AdjustScroll()

	if ln.ScrollOffset != 6 {
		t.Errorf("ScrollOffset: got %d, want 6 (selected - pageSize + 1)", ln.ScrollOffset)
	}
}

func TestAdjustScrollSelectedWithinViewport(t *testing.T) {
	ln := &ListNav{Selected: 3, ScrollOffset: 0, ListSize: 20, PageSize: 5}
	ln.AdjustScroll()

	if ln.ScrollOffset != 0 {
		t.Errorf("ScrollOffset: got %d, want 0 (no change needed)", ln.ScrollOffset)
	}
}

func TestAdjustScrollZeroPageSize(t *testing.T) {
	ln := &ListNav{Selected: 5, ScrollOffset: 0, ListSize: 10, PageSize: 0}
	ln.AdjustScroll() // should not panic or change anything

	if ln.ScrollOffset != 0 {
		t.Errorf("ScrollOffset should not change with PageSize=0")
	}
}

// --- SetListSize ---

func TestSetListSizeShrink(t *testing.T) {
	ln := New(10, 5)
	ln.Selected = 8
	ln.SetListSize(5)

	if ln.Selected != 4 {
		t.Errorf("Selected: got %d, want 4 (clamped)", ln.Selected)
	}
}

func TestSetListSizeToZero(t *testing.T) {
	ln := New(10, 5)
	ln.Selected = 5
	ln.SetListSize(0)

	if ln.Selected != 0 {
		t.Errorf("Selected: got %d, want 0", ln.Selected)
	}
	if ln.ScrollOffset != 0 {
		t.Errorf("ScrollOffset: got %d, want 0", ln.ScrollOffset)
	}
}

func TestSetListSizeGrow(t *testing.T) {
	ln := New(5, 3)
	ln.Selected = 4
	ln.SetListSize(20)

	if ln.Selected != 4 {
		t.Errorf("Selected: got %d, want 4 (unchanged)", ln.Selected)
	}
}

// --- SetPageSize ---

func TestSetPageSize(t *testing.T) {
	ln := New(20, 5)
	ln.Selected = 15
	ln.ScrollOffset = 10
	ln.SetPageSize(10)

	if ln.PageSize != 10 {
		t.Errorf("PageSize: got %d, want 10", ln.PageSize)
	}
	// Scroll should still keep selected visible
	if ln.Selected < ln.ScrollOffset || ln.Selected >= ln.ScrollOffset+ln.PageSize {
		t.Error("Selected should be within viewport after SetPageSize")
	}
}

func TestSetPageSizeMinimum(t *testing.T) {
	ln := New(10, 5)
	ln.SetPageSize(0)

	if ln.PageSize != 1 {
		t.Errorf("PageSize: got %d, want 1 (minimum)", ln.PageSize)
	}
}

// --- MoveDown triggers scroll ---

func TestMoveDownScrolls(t *testing.T) {
	ln := New(10, 3)
	// Move down past the viewport
	for range 5 {
		ln.MoveDown()
	}

	if ln.Selected != 5 {
		t.Errorf("Selected: got %d, want 5", ln.Selected)
	}
	// ScrollOffset should ensure selected is visible
	if ln.Selected < ln.ScrollOffset || ln.Selected >= ln.ScrollOffset+ln.PageSize {
		t.Errorf("Selected %d not visible: scrollOffset=%d, pageSize=%d",
			ln.Selected, ln.ScrollOffset, ln.PageSize)
	}
}

func TestMoveUpScrolls(t *testing.T) {
	ln := New(10, 3)
	ln.Selected = 8
	ln.ScrollOffset = 6

	for range 5 {
		ln.MoveUp()
	}

	if ln.Selected != 3 {
		t.Errorf("Selected: got %d, want 3", ln.Selected)
	}
	if ln.Selected < ln.ScrollOffset {
		t.Errorf("Selected %d is above viewport scrollOffset=%d", ln.Selected, ln.ScrollOffset)
	}
}
