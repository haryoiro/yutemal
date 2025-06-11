package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/haryoiro/yutemal/internal/structures"

	"github.com/mattn/go-runewidth"
)

// getStyles returns commonly used styles based on theme
func (m *Model) getStyles() (titleStyle, selectedStyle, normalStyle, dimStyle, errorStyle lipgloss.Style) {
	// Use theme manager if available, otherwise use defaults
	if m.themeManager != nil {
		titleStyle = m.themeManager.TitleStyle().MarginBottom(1)
		selectedStyle = m.themeManager.SelectedStyle().
			PaddingLeft(1).
			PaddingRight(1)
		normalStyle = m.themeManager.BaseStyle().
			PaddingLeft(1).
			PaddingRight(1)
		dimStyle = m.themeManager.SubtitleStyle()
		errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")).
			Bold(true)
	} else {
		// Fallback styles if theme manager is not initialized
		titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF79C6")).
			MarginBottom(1)
		selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8F8F2")).
			PaddingLeft(1).
			PaddingRight(1).
			Bold(true)
		normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8F8F2")).
			PaddingLeft(1).
			PaddingRight(1)
		dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4")).
			Italic(true)
		errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")).
			Bold(true)
	}
	return
}

func (m Model) renderPlaylistList(maxWidth int) string {
	titleStyle, selectedStyle, normalStyle, dimStyle, errorStyle := m.getStyles()

	var b strings.Builder
	b.WriteString(titleStyle.Render("🎵 Playlists"))
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("⚠️  Error: %v", m.err)))
		return b.String()
	}

	if len(m.playlists) == 0 {
		emptyHint := m.shortcutFormatter.GetEmptyStateHint("search", m.config.KeyBindings.Search)
		b.WriteString(dimStyle.Render("No playlists found.\n\n" + emptyHint))
		return b.String()
	}

	visibleItems := m.contentHeight - 4
	if visibleItems < 1 {
		visibleItems = 1
	}
	start := m.scrollOffset
	end := start + visibleItems
	if end > len(m.playlists) {
		end = len(m.playlists)
	}

	for i := start; i < end; i++ {
		playlist := m.playlists[i]
		icon := "📁"
		if i == m.selectedIndex {
			icon = "▶"
		}

		titleWidth := maxWidth - 8
		if titleWidth < 20 {
			titleWidth = 20
		}
		line := fmt.Sprintf("%s  %s", icon, truncate(playlist.Title, titleWidth))

		if i == m.selectedIndex {
			b.WriteString(selectedStyle.Render(line))
		} else {
			b.WriteString(normalStyle.Render(line))
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m Model) renderPlaylistDetail(maxWidth int) string {
	titleStyle, selectedStyle, normalStyle, dimStyle, _ := m.getStyles()

	// Apply focus style if this view has focus
	if m.hasFocus("playlist") {
		titleStyle = titleStyle.Underline(true)
	}

	var b strings.Builder

	// Header with title and shortcuts
	headerTitle := fmt.Sprintf("🎶 %s", m.playlistName)
	b.WriteString(titleStyle.Render(headerTitle))
	b.WriteString("\n\033[A")

	shortcuts := m.shortcutFormatter.FormatHints(m.shortcutFormatter.GetPlaylistHints())
	if runewidth.StringWidth(headerTitle) + runewidth.StringWidth(shortcuts) + 2 <= maxWidth {
		b.WriteString( dimStyle.Render(shortcuts))
	}
	b.WriteString("\033[B")
	b.WriteString("\n\n")

	if len(m.playlistTracks) == 0 {
		b.WriteString(dimStyle.Render("No tracks in this playlist"))
		return b.String()
	}

	visibleItems := m.contentHeight - 6 // Header and footer space
	if visibleItems < 1 {
		visibleItems = 1
	}
	start := m.playlistScrollOffset
	end := start + visibleItems
	if end > len(m.playlistTracks) {
		end = len(m.playlistTracks)
	}

	// 最小幅の確保と動的レイアウト調整
	if maxWidth < 50 {
		// 小さい画面用の簡略レイアウト
		titleWidth := maxWidth - 15
		if titleWidth < 10 {
			titleWidth = 10
		}

		for i := start; i < end; i++ {
			track := m.playlistTracks[i]
			status := " "

			// ダウンロード状態チェック
			if s, exists := m.playerState.MusicStatus[track.TrackID]; exists {
				switch s {
				case structures.Downloaded:
					status = "✓"
				case structures.Downloading:
					status = "↓"
				case structures.DownloadFailed:
					status = "✗"
				}
			}

			// 簡略表示（タイトルのみ）
			titleStr := truncate(track.Title, titleWidth)
			durationStr := formatDuration(track.Duration)

			// Track number or selection indicator
			trackNum := fmt.Sprintf("%2d. ", i+1)
			line := fmt.Sprintf("%s %s %s", status, titleStr, durationStr)

			// Apply style based on selection
			style := normalStyle
			if i == m.playlistSelectedIndex {
				trackNum = " →  "
				style = selectedStyle
			}

			line = trackNum + line
			b.WriteString(style.Render(line))

			if i < end-1 {
				b.WriteString("\n")
			}
		}

		// Simple footer for small screens
		b.WriteString("\n\n")
		positionInfo := fmt.Sprintf("%d/%d", m.playlistSelectedIndex+1, len(m.playlistTracks))
		b.WriteString(dimStyle.Render(positionInfo))

		return b.String()
	}

	// 通常のレイアウト（50文字以上）
	// 固定幅設定
	totalWidth := maxWidth - 4 // パディング分を考慮
	statusWidth := 2
	durationWidth := 7
	artistWidth := 25
	titleWidth := totalWidth - statusWidth - durationWidth - artistWidth - 6 // セパレーター分

	if titleWidth < 20 {
		titleWidth = 20
		artistWidth = 20
	}

	for i := start; i < end; i++ {
		track := m.playlistTracks[i]
		status := " "

		// ダウンロード状態チェック
		if s, exists := m.playerState.MusicStatus[track.TrackID]; exists {
			switch s {
			case structures.Downloaded:
				status = "✓"
			case structures.Downloading:
				status = "↓"
			case structures.DownloadFailed:
				status = "✗"
			}
		}

		// 各フィールドを固定幅でフォーマット
		titleStr := padToWidth(truncate(track.Title, titleWidth), titleWidth)
		artistStr := padToWidth(truncate(formatArtists(track.Artists), artistWidth), artistWidth)
		durationStr := formatDuration(track.Duration)

		// Track number or selection indicator
		trackNum := fmt.Sprintf("%3d. ", i+1)

		// Build line with fixed format
		line := fmt.Sprintf("%s%s %s %s",
			status,
			titleStr,
			artistStr,
			durationStr)

		// Apply style based on selection and current playing track
		style := normalStyle
		isCurrentTrack := false

		// Check if this track is currently playing
		if m.playerState.Current < len(m.playerState.List) && m.playerState.List[m.playerState.Current].TrackID == track.TrackID {
			isCurrentTrack = true
		}

		if isCurrentTrack {
			// Currently playing track
			trackNum = "  ▶  "
			style = selectedStyle
		} else if i == m.playlistSelectedIndex {
			// Selected track (when focused)
			trackNum = "  →  "
			style = selectedStyle.Background(lipgloss.Color("#44475A"))
		}

		line = trackNum + line
		b.WriteString(style.Render(line))

		if i < end-1 {
			b.WriteString("\n")
		}
	}

	// Footer with position info and help
	b.WriteString("\n\n")
	var footerInfo []string

	// Position info
	footerInfo = append(footerInfo, fmt.Sprintf("%d/%d", m.playlistSelectedIndex+1, len(m.playlistTracks)))

	// Navigation help
	navHints := m.shortcutFormatter.GetNavigationHints()
	navHints[1].Action = "play from here" // Override action text for this context
	footerInfo = append(footerInfo, m.shortcutFormatter.FormatHints(navHints))

	// Focus help
	focusHelp := m.getFocusHelpText()
	if focusHelp != "" {
		footerInfo = append(footerInfo, focusHelp)
	}

	b.WriteString(dimStyle.Render(strings.Join(footerInfo, "  ")))

	return b.String()
}

func (m Model) renderSearch(maxWidth int) string {
	titleStyle, selectedStyle, normalStyle, dimStyle, _ := m.getStyles()

	var b strings.Builder
	b.WriteString(titleStyle.Render("🔍 Search"))
	b.WriteString("\n")

	b.WriteString("Query: ")
	b.WriteString(m.searchQuery)
	b.WriteString("\n\n")

	if len(m.searchResults) == 0 {
		b.WriteString(dimStyle.Render("No results found."))
		return b.String()
	}

	visibleItems := m.contentHeight - 4
	if visibleItems < 1 {
		visibleItems = 1
	}
	start := m.scrollOffset
	end := start + visibleItems
	if end > len(m.searchResults) {
		end = len(m.searchResults)
	}

	// 最小幅の確保と動的レイアウト調整
	if maxWidth < 50 {
		// 小さい画面用のレイアウト
		titleWidth := maxWidth - 10
		if titleWidth < 10 {
			titleWidth = 10
		}

		for i := start; i < end; i++ {
			track := m.searchResults[i]

			// 簡略表示（タイトルのみ）
			titleStr := truncate(track.Title, titleWidth)
			line := titleStr

			if i == m.selectedIndex {
				b.WriteString(selectedStyle.Render(line))
			} else {
				b.WriteString(normalStyle.Render(line))
			}
			if i < end-1 {
				b.WriteString("\n")
			}
		}
	} else {
		// 通常のレイアウト
		// maxWidthは既にフレームサイズを考慮済み
		totalWidth := maxWidth - 4 // パディング分のみ考慮
		durationWidth := 7
		artistWidth := 25
		titleWidth := totalWidth - durationWidth - artistWidth - 4 // セパレーター分

		if titleWidth < 30 {
			titleWidth = 30
			artistWidth = 20
		}

		for i := start; i < end; i++ {
			track := m.searchResults[i]

			// 各フィールドを固定幅でフォーマット
			titleStr := padToWidth(truncate(track.Title, titleWidth), titleWidth)
			artistStr := padToWidth(truncate(formatArtists(track.Artists), artistWidth), artistWidth)
			durationStr := formatDuration(track.Duration)

			// 固定フォーマットで行を構築
			line := fmt.Sprintf("%s %s %s",
				titleStr,
				artistStr,
				durationStr)

			if i == m.selectedIndex {
				b.WriteString(selectedStyle.Render(line))
			} else {
				b.WriteString(normalStyle.Render(line))
			}
			if i < end-1 {
				b.WriteString("\n")
			}
		}
	}

	return b.String()
}

// renderHome renders the home view with sections
func (m Model) renderHome(maxWidth int) string {
	_, selectedStyle, normalStyle, dimStyle, errorStyle := m.getStyles()

	var b strings.Builder

	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("⚠️  Error: %v", m.err)))
		return b.String()
	}

	if len(m.sections) == 0 {
		b.WriteString(" "+dimStyle.Render("Loading home page..."))
		return b.String()
	}

	// レンダリングするセクションタブ
	b.WriteString(m.renderSectionTabs(maxWidth))
	b.WriteString("\n\n")

	// 現在のセクションのコンテンツをレンダリング
	if m.currentSectionIndex < len(m.sections) {
		section := m.sections[m.currentSectionIndex]

		if len(section.Contents) == 0 {
			b.WriteString(dimStyle.Render("No content in this section"))
			return b.String()
		}

		visibleItems := m.contentHeight - 8 // タブとタイトル用のスペースを確保
		if visibleItems < 1 {
			visibleItems = 1
		}

		startIndex := m.scrollOffset
		endIndex := startIndex + visibleItems
		if endIndex > len(section.Contents) {
			endIndex = len(section.Contents)
		}

		for i := startIndex; i < endIndex; i++ {
			content := section.Contents[i]
			style := normalStyle
			prefix := "   "

			if i == m.selectedIndex {
				style = selectedStyle
				prefix = " ▶ "
			}

			var displayText string
			switch content.Type {
			case "playlist":
				if content.Playlist != nil {
					displayText = fmt.Sprintf("📁 %s", content.Playlist.Title)
					if content.Playlist.VideoCount > 0 {
						displayText += fmt.Sprintf(" (%d tracks)", content.Playlist.VideoCount)
					}
				}
			case "track":
				if content.Track != nil {
					artists := strings.Join(content.Track.Artists, ", ")
					displayText = fmt.Sprintf("🎵 %s - %s", content.Track.Title, artists)
				}
			default:
				displayText = fmt.Sprintf("Unknown content type: %s", content.Type)
			}

			// 長すぎるテキストを切り詰める
			availableWidth := maxWidth - runewidth.StringWidth(prefix) - 2
			if availableWidth > 0 && runewidth.StringWidth(displayText) > availableWidth {
				if availableWidth > 3 {
					// 文字列を切り詰め
					runes := []rune(displayText)
					truncated := ""
					width := 0
					for _, r := range runes {
						charWidth := runewidth.RuneWidth(r)
						if width+charWidth > availableWidth-3 {
							break
						}
						truncated += string(r)
						width += charWidth
					}
					displayText = truncated + "..."
				} else {
					displayText = "..."
				}
			}

			b.WriteString(style.Render(prefix + displayText))
			if i < endIndex-1 {
				b.WriteString("\n")
			}
		}

		// スクロールインジケーター
		if len(section.Contents) > visibleItems {
			totalItems := len(section.Contents)
			currentPage := (m.selectedIndex / visibleItems) + 1
			totalPages := (totalItems + visibleItems - 1) / visibleItems
			b.WriteString("\n\n")
			b.WriteString(dimStyle.Render(fmt.Sprintf("Page %d/%d (%d items)", currentPage, totalPages, totalItems)))
		}
	}

	return b.String()
}

// renderSectionTabs renders the section tabs at the top
func (m Model) renderSectionTabs(maxWidth int) string {
	titleStyle, selectedStyle, normalStyle, dimStyle, _ := m.getStyles()

	// Apply focus style if home view has focus
	if m.hasFocus("home") {
		titleStyle = titleStyle.Underline(true)
	}

	if len(m.sections) <= 1 {
		return ""
	}

	var tabs []string
	for i, section := range m.sections {
		tabStyle := normalStyle.Copy().PaddingLeft(2).PaddingRight(2)

		if i == m.currentSectionIndex {
			tabStyle = selectedStyle.Copy().PaddingLeft(2).PaddingRight(2)
		}

		tabs = append(tabs, tabStyle.Render(section.Title))
	}

	tabsStr := strings.Join(tabs, " ")

	// タブがウィンドウ幅を超える場合の処理
	if runewidth.StringWidth(tabsStr) > maxWidth {
		// 簡単な実装：現在のタブだけを表示
		currentTab := selectedStyle.Copy().PaddingLeft(2).PaddingRight(2).Render(m.sections[m.currentSectionIndex].Title)
		return currentTab + dimStyle.Render(" " + m.shortcutFormatter.GetSectionNavigationHint(true))
	}

	if m.showQueue {
		return tabsStr + "\n" + dimStyle.Render(m.shortcutFormatter.GetSectionNavigationHint(false))
	}
	return tabsStr + "\n  " + dimStyle.Render("Tab to switch sections")
}

func (m Model) applyMarquee(text string, maxLen int) string {
	textWidth := runewidth.StringWidth(text)
	if textWidth <= maxLen {
		return text
	}

	// Convert to runes for proper Unicode handling
	runes := []rune(text)
	spacer := []rune("     ") // 5スペースのセパレータ

	// Create padded text with spacer
	paddedRunes := append(append([]rune{}, runes...), spacer...)
	paddedRunes = append(paddedRunes, runes...) // タイトルを繰り返す

	// スクロール速度を調整 - テキストの長さに応じて動的に調整
	// 長いテキストほど遅くスクロールする
	textLength := len(runes)
	scrollDivisor := 3 // デフォルトの速度調整値

	// テキストの長さに基づいて速度を調整
	if textLength > 30 {
		scrollDivisor = 4
	}
	if textLength > 60 {
		scrollDivisor = 5
	}
	if textLength > 90 {
		scrollDivisor = 6
	}
	if textLength > 120 {
		scrollDivisor = 7
	}

	effectiveOffset := m.marqueeOffset / scrollDivisor

	// Calculate offset based on rune count
	totalRunes := len(paddedRunes)
	offset := effectiveOffset % totalRunes

	// Build result string with proper width calculation
	var result []rune
	currentWidth := 0

	// Start from offset position
	for i := offset; currentWidth < maxLen && i < totalRunes; i++ {
		r := paddedRunes[i]
		w := runewidth.RuneWidth(r)

		// Check if adding this rune would exceed maxLen
		if currentWidth+w > maxLen {
			// 最後の文字が切れる場合はスペースで埋める
			for currentWidth < maxLen {
				result = append(result, ' ')
				currentWidth++
			}
			break
		}

		result = append(result, r)
		currentWidth += w
	}

	// If we need more characters, wrap around to the beginning
	if currentWidth < maxLen {
		for i := 0; currentWidth < maxLen && i < offset; i++ {
			r := paddedRunes[i]
			w := runewidth.RuneWidth(r)

			if currentWidth+w > maxLen {
				// 最後の文字が切れる場合はスペースで埋める
				for currentWidth < maxLen {
					result = append(result, ' ')
					currentWidth++
				}
				break
			}

			result = append(result, r)
			currentWidth += w
		}
	}

	// Pad with spaces if needed to maintain consistent width
	for currentWidth < maxLen {
		result = append(result, ' ')
		currentWidth++
	}

	return string(result)
}

// isASCII checks if a string contains only ASCII characters
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func truncate(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}

	currentWidth := runewidth.StringWidth(s)
	if currentWidth <= maxWidth {
		return s
	}

	// 省略記号用のスペースを確保
	if maxWidth <= 3 {
		// Very short max width - just return the first few bytes
		if len(s) <= maxWidth {
			return s
		}
		// For ASCII, we can use simple byte slicing
		if isASCII(s) && len(s) > maxWidth {
			return s[:maxWidth]
		}
		// For non-ASCII, need proper rune handling
		runes := []rune(s)
		if len(runes) <= maxWidth {
			return s
		}
		return string(runes[:maxWidth])
	}

	// 省略記号込みで切り詰め
	targetWidth := maxWidth - 3 // "..."分を引く
	runes := []rune(s)
	result := make([]rune, 0, len(runes))
	width := 0

	for _, r := range runes {
		rw := runewidth.RuneWidth(r)
		if width + rw > targetWidth {
			break
		}
		result = append(result, r)
		width += rw
	}

	return string(result) + "..."
}

func formatDuration(seconds int) string {
	if seconds <= 0 {
		return "--:--"
	}
	minutes := seconds / 60
	secs := seconds % 60
	return fmt.Sprintf("%02d:%02d", minutes, secs)
}

func formatArtists(artists []string) string {
	if len(artists) == 0 {
		return "Unknown Artist"
	}

	// 長すぎる場合は最初のアーティストのみ表示
	if len(artists) == 1 {
		return artists[0]
	}

	// 複数アーティストの場合
	result := artists[0]
	for i := 1; i < len(artists); i++ {
		testResult := result + ", " + artists[i]
		// 仮の最大幅をチェック（実際の幅は呼び出し元で調整）
		if runewidth.StringWidth(testResult) > 50 {
			result += ", ..."
			break
		}
		result = testResult
	}

	return result
}

func padToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}

	currentWidth := runewidth.StringWidth(s)
	if currentWidth >= width {
		return s
	}

	// 不足分をスペースで埋める
	padding := width - currentWidth
	return s + strings.Repeat(" ", padding)
}

// renderQueue renders the queue panel on the right side
func (m *Model) renderQueue(maxWidth int, maxHeight int) string {
	titleStyle, selectedStyle, normalStyle, dimStyle, _ := m.getStyles()

	// Apply focus style
	if m.hasFocus("queue") {
		titleStyle = titleStyle.Underline(true)
	}

	var b strings.Builder

	// Header
	queueTitle := "🎵 Queue"
	b.WriteString(titleStyle.Render(queueTitle))
	b.WriteString("\n\033[A")

	hints := m.shortcutFormatter.GetQueueHints(m.hasFocus("queue"))
	if len(hints) > 0 {
		shortcuts := m.shortcutFormatter.FormatHint(hints[0])
		if runewidth.StringWidth(queueTitle) + runewidth.StringWidth(shortcuts) + 2 <= maxWidth {
			b.WriteString(dimStyle.Render(shortcuts))
		}
	}
	b.WriteString("\033[B")
	b.WriteString("\n\n")

	// If no tracks in queue
	if len(m.playerState.List) == 0 {
		b.WriteString(dimStyle.Render("No tracks in queue"))
		return b.String()
	}

	// Calculate visible lines (excluding header)
	visibleLines := maxHeight - 4 // Header, spacing, and scroll indicator
	if visibleLines < 1 {
		visibleLines = 1
	}

	// Ensure selected item is visible when queue is focused
	if m.queueFocused {
		if m.queueSelectedIndex < m.queueScrollOffset {
			m.queueScrollOffset = m.queueSelectedIndex
		} else if m.queueSelectedIndex >= m.queueScrollOffset+visibleLines {
			m.queueScrollOffset = m.queueSelectedIndex - visibleLines + 1
		}
	}

	// Ensure scroll offset is valid
	maxScrollOffset := len(m.playerState.List) - visibleLines
	if maxScrollOffset < 0 {
		maxScrollOffset = 0
	}
	if m.queueScrollOffset > maxScrollOffset {
		m.queueScrollOffset = maxScrollOffset
	}

	// Render tracks
	startIndex := m.queueScrollOffset
	endIndex := startIndex + visibleLines
	if endIndex > len(m.playerState.List) {
		endIndex = len(m.playerState.List)
	}

	// Get actual track indices
	getTrackIndex := func(displayIndex int) int {
		return displayIndex
	}

	for displayIdx := startIndex; displayIdx < endIndex; displayIdx++ {
		actualIdx := getTrackIndex(displayIdx)
		if actualIdx >= len(m.playerState.List) {
			continue
		}

		track := m.playerState.List[actualIdx]

		// Format track info
		artists := formatArtists(track.Artists)
		title := track.Title

		// Add status icon
		var statusIcon string
		if status, ok := m.playerState.MusicStatus[track.TrackID]; ok {
			switch status {
			case structures.Downloaded:
				statusIcon = "✓ "
			case structures.Downloading:
				statusIcon = "⬇ "
			case structures.DownloadFailed:
				statusIcon = "✗ "
			default:
				statusIcon = "○ "
			}
		} else {
			statusIcon = "○ "
		}

		// Format line with track number
		trackNum := fmt.Sprintf("%2d. ", displayIdx+1)
		line := fmt.Sprintf("%s%s - %s", statusIcon, title, artists)

		// Truncate if too long
		availableWidth := maxWidth - runewidth.StringWidth(trackNum) - 4 // Track number and padding
		line = truncate(line, availableWidth)

		// Apply style based on selection and current track
		style := normalStyle
		isCurrentTrack := actualIdx == m.playerState.Current

		if isCurrentTrack {
			// Current playing track
			trackNum = "▶   "
			style = selectedStyle
		} else if m.hasFocus("queue") && displayIdx == m.queueSelectedIndex {
			// Selected track in queue (when focused)
			trackNum = "→   "
			style = selectedStyle.Background(lipgloss.Color("#44475A"))
		}

		line = trackNum + line

		b.WriteString(style.Render(line))
		if displayIdx < endIndex-1 {
			b.WriteString("\n")
		}
	}

	// Scroll indicator and help
	if len(m.playerState.List) > visibleLines || m.queueFocused {
		b.WriteString("\n\n")
		var info []string

		// Position info
		info = append(info, fmt.Sprintf("%d/%d", m.playerState.Current+1, len(m.playerState.List)))

		// Help text when focused
		if m.hasFocus("queue") {
			hints := m.shortcutFormatter.GetQueueHints(true)
			if len(hints) > 1 {
				info = append(info, m.shortcutFormatter.FormatHints(hints[1:])) // Skip the Tab hint
			}
		}

		b.WriteString(dimStyle.Render(strings.Join(info, " ")))
	}

	return b.String()
}
