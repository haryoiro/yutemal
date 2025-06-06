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
		b.WriteString(dimStyle.Render("No playlists found.\n\nPress 'f' to search"))
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

	var b strings.Builder
	b.WriteString(titleStyle.Render("🎶 PLAYLIST: " + m.currentListName))
	b.WriteString("\n")

	if len(m.currentList) == 0 {
		emptyMessage := dimStyle.Render("No songs in this playlist")
		b.WriteString(emptyMessage)
		return b.String()
	}

	visibleItems := m.contentHeight
	if visibleItems < 1 {
		visibleItems = 1
	}
	start := m.scrollOffset
	end := start + visibleItems
	if end > len(m.currentList) {
		end = len(m.currentList)
	}

	// 最小幅の確保と動的レイアウト調整
	if maxWidth < 50 {
		// 小さい画面用の簡略レイアウト
		titleWidth := maxWidth - 15
		if titleWidth < 10 {
			titleWidth = 10
		}

		for i := start; i < end; i++ {
			track := m.currentList[i]
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

			line := fmt.Sprintf("%s %s %s", status, titleStr, durationStr)

			if i == m.selectedIndex {
				b.WriteString(selectedStyle.Render(line))
			} else {
				b.WriteString(normalStyle.Render(line))
			}
		}
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
		track := m.currentList[i]
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

		// 固定フォーマットで行を構築
		line := fmt.Sprintf("%s %s %s %s",
			status,
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
	titleStyle, selectedStyle, normalStyle, dimStyle, errorStyle := m.getStyles()

	var b strings.Builder

	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("⚠️  Error: %v", m.err)))
		return b.String()
	}

	if len(m.sections) == 0 {
		b.WriteString(dimStyle.Render("Loading home page..."))
		return b.String()
	}

	// レンダリングするセクションタブ
	b.WriteString(m.renderSectionTabs(maxWidth))
	b.WriteString("\n\n")

	// 現在のセクションのコンテンツをレンダリング
	if m.currentSectionIndex < len(m.sections) {
		section := m.sections[m.currentSectionIndex]
		b.WriteString(titleStyle.Render(fmt.Sprintf("📁 %s", section.Title)))
		b.WriteString("\n")

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
	_, selectedStyle, normalStyle, dimStyle, _ := m.getStyles()

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
		return currentTab + dimStyle.Render(" (Tab/Shift+Tab to switch)")
	}

	return tabsStr + "\n" + dimStyle.Render("Tab/Shift+Tab to switch sections")
}

func (m Model) applyMarquee(text string, maxLen int) string {
	textWidth := runewidth.StringWidth(text)
	if textWidth <= maxLen {
		return text
	}

	// Convert to runes for proper Unicode handling
	runes := []rune(text)
	spacer := []rune("   ")
	
	// Create padded text with spacer
	paddedRunes := append(append([]rune{}, runes...), spacer...)
	paddedRunes = append(paddedRunes, runes...)
	
	// Calculate offset based on rune count
	totalRunes := len(paddedRunes)
	offset := m.marqueeOffset % totalRunes
	
	// Build result string with proper width calculation
	var result []rune
	currentWidth := 0
	
	for i := offset; currentWidth < maxLen && i < totalRunes; i++ {
		r := paddedRunes[i]
		w := runewidth.RuneWidth(r)
		
		// Check if adding this rune would exceed maxLen
		if currentWidth + w > maxLen {
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
			
			if currentWidth + w > maxLen {
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
		// 短すぎる場合は文字単位で切り詰め
		runes := []rune(s)
		result := ""
		for _, r := range runes {
			testStr := result + string(r)
			if runewidth.StringWidth(testStr) > maxWidth {
				break
			}
			result = testStr
		}
		return result
	}

	// 省略記号込みで切り詰め
	targetWidth := maxWidth - 3 // "..."分を引く
	runes := []rune(s)
	result := ""

	for _, r := range runes {
		testStr := result + string(r)
		if runewidth.StringWidth(testStr) > targetWidth {
			break
		}
		result = testStr
	}

	return result + "..."
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
