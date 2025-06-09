package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/haryoiro/yutemal/internal/structures"
	"github.com/mattn/go-runewidth"
)

func (m *Model) renderPlayer() string {
	// Get styles from theme manager
	var playerInfoStyle, timeStyle, dimStyle lipgloss.Style

	if m.themeManager != nil {
		playerInfoStyle = m.themeManager.TitleStyle()
		timeStyle = m.themeManager.BaseStyle().Foreground(lipgloss.Color(m.config.Theme.Selected))
		dimStyle = m.themeManager.SubtitleStyle()
	} else {
		// Fallback styles
		playerInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8F8F2")).
			Bold(true)
		timeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8BE9FD"))
		dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4")).
			Italic(true)
	}

	// 利用可能幅を使用（フレームサイズは既に考慮済み）
	availableWidth := m.playerContentWidth
	if availableWidth <= 0 {
		availableWidth = 80 // フォールバック値
	}

	// Use full width for content (thumbnail removed)
	contentWidth := availableWidth

	// Content rendering
	var content strings.Builder

	// First line: Song info
	if m.playerState.Current < len(m.playerState.List) && m.playerState.Current >= 0 {
		video := m.playerState.List[m.playerState.Current]
		title := video.Title
		artists := formatArtists(video.Artists)

		// タイトルとアーティストの表示を最適化
		prefixWidth := runewidth.StringWidth("🎵 ")
		separatorWidth := runewidth.StringWidth(" - ")
		artistsWidth := runewidth.StringWidth(artists)

		// 利用可能な全体幅からアーティスト名とセパレータの分を引いてタイトル幅を決定
		// 少し余裕を持たせる（-2）
		maxTitleWidth := contentWidth - prefixWidth - separatorWidth - artistsWidth - 2

		// 最小幅を確保
		if maxTitleWidth < 20 {
			// タイトルが短すぎる場合はアーティスト名を短縮
			maxTitleWidth = contentWidth * 2 / 3 // 全体の2/3をタイトルに
			maxArtistWidth := contentWidth - prefixWidth - separatorWidth - maxTitleWidth - 2
			if maxArtistWidth > 0 {
				artists = truncate(artists, maxArtistWidth)
			}
		}

		// タイトルが長い場合はマーキー表示
		titleWidth := runewidth.StringWidth(title)
		if titleWidth > maxTitleWidth {
			title = m.applyMarquee(title, maxTitleWidth)
		}

		// 最終的な表示文字列を構築
		displayString := fmt.Sprintf("🎵 %s - %s", title, artists)

		// 幅が超過している場合の最終調整
		actualWidth := runewidth.StringWidth(displayString)
		if actualWidth > contentWidth {
			// オーバーフローしている分を計算
			overflow := actualWidth - contentWidth
			// アーティスト名から削る
			newArtistWidth := runewidth.StringWidth(artists) - overflow - 2
			if newArtistWidth > 0 {
				artists = truncate(artists, newArtistWidth)
				displayString = fmt.Sprintf("🎵 %s - %s", title, artists)
			} else {
				// アーティスト名を完全に省略
				displayString = truncate(fmt.Sprintf("🎵 %s", title), contentWidth)
			}
		}

		content.WriteString(playerInfoStyle.Render(displayString))
	} else {
		content.WriteString(dimStyle.Render("NO SONG PLAYING"))
	}
	content.WriteString("\n\n")

	// Second line: Progress bar
	if m.playerState.TotalTime > 0 {
		currentTime := formatDuration(int(m.playerState.CurrentTime.Seconds()))
		totalTime := formatDuration(int(m.playerState.TotalTime.Seconds()))

		// Calculate exact width needed for time displays and spacing
		timeWidth := runewidth.StringWidth(currentTime) + runewidth.StringWidth(totalTime) // 2 spaces
		barWidth := contentWidth - timeWidth*2 + 6
		if barWidth < 10 {
			barWidth = 10
		}

		progressBar := m.renderProgressBar(barWidth)

		content.WriteString(fmt.Sprintf("%s %s %s",
			timeStyle.Render(currentTime),
			progressBar,
			timeStyle.Render(totalTime)))
	} else {
		// Get progressBgStyle for empty player
		var progressBgStyle lipgloss.Style
		if m.themeManager != nil {
			progressBgStyle = m.themeManager.ProgressStyle()
		} else {
			progressBgStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#44475A"))
		}

		// Calculate exact width for empty progress bar
		timeWidth := runewidth.StringWidth("--:--") * 2 // 2 time displays + 2 spaces
		barWidth := contentWidth - timeWidth*2 + 6
		if barWidth < 10 {
			barWidth = 10
		}
		bar := progressBgStyle.Render(strings.Repeat("─", barWidth))
		content.WriteString(fmt.Sprintf("%s %s %s",
			timeStyle.Render("--:--"),
			bar,
			timeStyle.Render("--:--")))
	}
	content.WriteString("\n\n")

	// Third line: Controls and status
	controls := m.renderControls(contentWidth)
	content.WriteString(controls)

	return content.String()
}

func (m *Model) renderProgressBar(width int) string {
	// Get styles
	var progressBarStyle, progressBgStyle lipgloss.Style
	if m.themeManager != nil {
		progressBarStyle = m.themeManager.ProgressFillStyle()
		progressBgStyle = m.themeManager.ProgressStyle()
	} else {
		progressBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#50FA7B"))
		progressBgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#44475A"))
	}

	// Ensure minimum width
	if width < 10 {
		width = 10
	}

	progress := float64(m.playerState.CurrentTime) / float64(m.playerState.TotalTime)
	if progress > 1 {
		progress = 1
	}
	if progress < 0 {
		progress = 0
	}

	filled := int(float64(width) * progress)
	empty := width - filled

	// Choose style based on config
	style := m.config.Theme.ProgressBarStyle
	if style == "" {
		style = "gradient" // Default to gradient
	}

	bar := strings.Builder{}

	switch style {
	case "block":
		// Block style with solid blocks
		if filled > 0 {
			bar.WriteString(progressBarStyle.Render(strings.Repeat("█", filled)))
		}
		if empty > 0 {
			bar.WriteString(progressBgStyle.Render(strings.Repeat("░", empty)))
		}

	case "line":
		// Line style with simple lines
		if filled > 0 {
			bar.WriteString(progressBarStyle.Render(strings.Repeat("─", filled)))
		}
		if empty > 0 {
			bar.WriteString(progressBgStyle.Render(strings.Repeat("─", empty)))
		}

	case "gradient":
		// Gradient style with smooth transition
		if filled > 0 {
			// Create gradient effect
			gradientBar := m.createGradientBar(filled, m.config.Theme.ProgressBar, m.config.Theme.ProgressBarFill)
			bar.WriteString(gradientBar)
		}
		if empty > 0 {
			bar.WriteString(progressBgStyle.Render(strings.Repeat("━", empty)))
		}

	default:
		// Default to gradient
		if filled > 0 {
			gradientBar := m.createGradientBar(filled, m.config.Theme.ProgressBar, m.config.Theme.ProgressBarFill)
			bar.WriteString(gradientBar)
		}
		if empty > 0 {
			bar.WriteString(progressBgStyle.Render(strings.Repeat("━", empty)))
		}
	}

	return bar.String()
}

// createGradientBar creates a gradient effect between two colors
func (m *Model) createGradientBar(width int, startColor, endColor string) string {
	if width <= 0 {
		return ""
	}

	// For simplicity, we'll create a simple 3-step gradient
	// In a real implementation, you could interpolate colors more smoothly
	result := ""

	if width == 1 {
		// Just use the end color for single character
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(endColor))
		result = style.Render("━")
	} else if width == 2 {
		// Use start and end colors
		startStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(startColor))
		endStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(endColor))
		result = startStyle.Render("━") + endStyle.Render("━")
	} else {
		// Create a simple gradient with start, middle (mixed), and end
		startStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(startColor))
		endStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(endColor))

		// For the middle section, use the end color but slightly dimmed
		// This creates a visual gradient effect
		middleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(endColor)).Faint(true)

		startLen := width / 3
		endLen := width / 3
		middleLen := width - startLen - endLen

		if startLen > 0 {
			result += startStyle.Render(strings.Repeat("━", startLen))
		}
		if middleLen > 0 {
			result += middleStyle.Render(strings.Repeat("━", middleLen))
		}
		if endLen > 0 {
			result += endStyle.Render(strings.Repeat("━", endLen))
		}
	}

	return result
}

func (m *Model) renderControls(availableWidth int) string {
	// Get dimStyle
	var dimStyle lipgloss.Style
	if m.themeManager != nil {
		dimStyle = m.themeManager.SubtitleStyle()
	} else {
		dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4")).
			Italic(true)
	}

	var parts []string

	// Play/Pause status
	if m.playerState.IsPlaying {
		parts = append(parts, "▶ Playing")
	} else {
		parts = append(parts, "⏸ Paused")
	}

	// Volume
	volume := int(m.playerState.Volume * 100)
	volumeIcon := "🔊"
	if volume == 0 {
		volumeIcon = "🔇"
	} else if volume < 30 {
		volumeIcon = "🔈"
	} else if volume < 70 {
		volumeIcon = "🔉"
	}
	parts = append(parts, fmt.Sprintf("%s %d%%", volumeIcon, volume))

	// Bitrate info
	if m.playerState.Current < len(m.playerState.List) && m.playerState.Current >= 0 {
		track := m.playerState.List[m.playerState.Current]
		if track.AudioBitrate > 0 {
			parts = append(parts, fmt.Sprintf("🎵 %d kbps", track.AudioBitrate))
		} else if track.AudioQuality != "" {
			parts = append(parts, fmt.Sprintf("🎵 %s", track.AudioQuality))
		}
	}

	// Download status
	if m.playerState.Current < len(m.playerState.List) && m.playerState.Current >= 0 {
		video := m.playerState.List[m.playerState.Current]
		if status, exists := m.playerState.MusicStatus[video.TrackID]; exists {
			if status == structures.Downloading {
				parts = append(parts, "⬇️  Downloading")
			}
		}
	}

	// Controls hint
	hint := "[Space: Play/Pause] [←/→: Seek] [s: Shuffle] [q: Queue] [Ctrl+D: Quit]"
	parts = append(parts, dimStyle.Render(hint))

	// 利用可能幅に収まるように調整
	fullLine := strings.Join(parts, "  ")
	if runewidth.StringWidth(fullLine) > availableWidth {
		// ヒント部分を短縮
		withoutHint := strings.Join(parts[:len(parts)-1], "  ")
		remaining := availableWidth - runewidth.StringWidth(withoutHint) - 2
		if remaining > 10 {
			truncatedHint := truncate(hint, remaining)
			parts[len(parts)-1] = dimStyle.Render(truncatedHint)
		} else {
			parts = parts[:len(parts)-1]
		}
		fullLine = strings.Join(parts, "  ")
	}

	return fullLine
}
