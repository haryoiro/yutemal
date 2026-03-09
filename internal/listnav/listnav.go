package listnav

// ListNav manages selection and scroll state for a scrollable list.
// All methods are pure state manipulation with no I/O or UI dependencies.
type ListNav struct {
	Selected     int
	ScrollOffset int
	ListSize     int
	PageSize     int
}

// New creates a ListNav with the given list size and page size.
func New(listSize, pageSize int) *ListNav {
	return &ListNav{
		ListSize: listSize,
		PageSize: pageSize,
	}
}

// MoveUp moves selection up by one. Returns true if moved.
func (ln *ListNav) MoveUp() bool {
	if ln.Selected <= 0 {
		return false
	}
	ln.Selected--
	ln.AdjustScroll()
	return true
}

// MoveDown moves selection down by one. Returns true if moved.
func (ln *ListNav) MoveDown() bool {
	if ln.Selected >= ln.ListSize-1 {
		return false
	}
	ln.Selected++
	ln.AdjustScroll()
	return true
}

// PageUp moves selection up by one page.
func (ln *ListNav) PageUp() {
	ln.Selected -= ln.PageSize
	if ln.Selected < 0 {
		ln.Selected = 0
	}
	ln.AdjustScroll()
}

// PageDown moves selection down by one page.
func (ln *ListNav) PageDown() {
	ln.Selected += ln.PageSize
	max := ln.ListSize - 1
	if max < 0 {
		max = 0
	}
	if ln.Selected > max {
		ln.Selected = max
	}
	ln.AdjustScroll()
}

// JumpToTop moves selection to the first item.
func (ln *ListNav) JumpToTop() {
	ln.Selected = 0
	ln.ScrollOffset = 0
}

// JumpToBottom moves selection to the last item.
func (ln *ListNav) JumpToBottom() {
	if ln.ListSize > 0 {
		ln.Selected = ln.ListSize - 1
	} else {
		ln.Selected = 0
	}
	ln.AdjustScroll()
}

// AdjustScroll adjusts scroll offset to keep the selected item visible within the viewport.
func (ln *ListNav) AdjustScroll() {
	if ln.PageSize <= 0 {
		return
	}
	if ln.Selected < ln.ScrollOffset {
		ln.ScrollOffset = ln.Selected
	} else if ln.Selected >= ln.ScrollOffset+ln.PageSize {
		ln.ScrollOffset = ln.Selected - ln.PageSize + 1
	}
}

// SetListSize updates the list size and clamps selection if needed.
func (ln *ListNav) SetListSize(size int) {
	ln.ListSize = size
	if size == 0 {
		ln.Selected = 0
		ln.ScrollOffset = 0
		return
	}
	if ln.Selected >= size {
		ln.Selected = size - 1
	}
	ln.AdjustScroll()
}

// SetPageSize updates the page size.
func (ln *ListNav) SetPageSize(size int) {
	if size < 1 {
		size = 1
	}
	ln.PageSize = size
	ln.AdjustScroll()
}
