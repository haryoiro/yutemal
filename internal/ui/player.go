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

	var b strings.Builder

	// 利用可能幅を使用（フレームサイズは既に考慮済み）
	availableWidth := m.playerContentWidth
	if availableWidth <= 0 {
		availableWidth = 80 // フォールバック値
	}

	// First line: Song info
	if m.playerState.Current < len(m.playerState.List) && m.playerState.Current >= 0 {
		video := m.playerState.List[m.playerState.Current]
		title := video.Title
		artists := formatArtists(video.Artists)

		// タイトルとアーティストの幅計算
		artistsWidth := runewidth.StringWidth(artists)
		prefixWidth := runewidth.StringWidth("🎵  - ")
		maxTitleWidth := availableWidth - artistsWidth - prefixWidth

		if maxTitleWidth < 10 {
			maxTitleWidth = 10
		}

		titleWidth := runewidth.StringWidth(title)
		if titleWidth > maxTitleWidth {
			title = m.applyMarquee(title, maxTitleWidth)
		}

		// 全体の行が利用可能幅に収まるように調整
		fullLine := fmt.Sprintf("🎵 %s - %s", title, artists)
		if runewidth.StringWidth(fullLine) > availableWidth {
			maxArtistsWidth := availableWidth - runewidth.StringWidth(fmt.Sprintf("🎵 %s - ", title))
			if maxArtistsWidth > 0 {
				artists = truncate(artists, maxArtistsWidth)
			}
		}

		b.WriteString(playerInfoStyle.Render(fmt.Sprintf("🎵 %s - %s", title, artists)))
	} else {
		b.WriteString(dimStyle.Render("NO SONG PLAYING"))
	}
	b.WriteString("\n\n")

	// Second line: Progress bar
	if m.playerState.TotalTime > 0 {
		currentTime := formatDuration(int(m.playerState.CurrentTime.Seconds()))
		totalTime := formatDuration(int(m.playerState.TotalTime.Seconds()))

		// Calculate exact width needed for time displays and spacing
		timeWidth := runewidth.StringWidth(currentTime) + runewidth.StringWidth(totalTime) + 2 // 2 spaces
		barWidth := availableWidth - timeWidth*2
		if barWidth < 10 {
			barWidth = 10
		}

		progressBar := m.renderProgressBar(barWidth)

		b.WriteString(fmt.Sprintf("%s %s %s",
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
		timeWidth := runewidth.StringWidth("--:--")*2 + 2 // 2 time displays + 2 spaces
		barWidth := availableWidth - timeWidth*2
		if barWidth < 10 {
			barWidth = 10
		}
		bar := progressBgStyle.Render(strings.Repeat("─", barWidth))
		b.WriteString(fmt.Sprintf("%s %s %s",
			timeStyle.Render("--:--"),
			bar,
			timeStyle.Render("--:--")))
	}
	b.WriteString("\n\n")

	// Third line: Controls and status
	controls := m.renderControls(availableWidth)
	b.WriteString(controls)

	return b.String()
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
	hint := "[Space: Play/Pause] [←/→: Seek] [+/-: Volume] [Ctrl+D: Quit]"
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
