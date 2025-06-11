package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	runewidth "github.com/mattn/go-runewidth"

	"github.com/haryoiro/yutemal/internal/structures"
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
		prefixWidth := MusicEmojiWidth
		separatorWidth := SeparatorWidth
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
			m.needsMarquee = true
			title = m.applyMarquee(title, maxTitleWidth)
		} else {
			m.needsMarquee = false
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
		// Time format is always "MM:SS" so both are 5 characters
		timeWidth := 10 + 2 // Two time displays (5 chars each) plus 2 spaces

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
		timeWidth := TimeFormatWidth*2 + 2 // 2 time displays + 2 spaces

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
			bar.WriteString(progressBarStyle.Render(strings.Repeat(ProgressBlockFilled, filled)))
		}

		if empty > 0 {
			bar.WriteString(progressBgStyle.Render(strings.Repeat(ProgressBlockEmpty, empty)))
		}

	case "line":
		// Line style with simple lines
		if filled > 0 {
			bar.WriteString(progressBarStyle.Render(strings.Repeat(ProgressLineFilled, filled)))
		}

		if empty > 0 {
			bar.WriteString(progressBgStyle.Render(strings.Repeat(ProgressLineEmpty, empty)))
		}

	case "gradient":
		// Gradient style with smooth transition
		if filled > 0 {
			// Create gradient effect
			gradientBar := m.createGradientBar(filled, m.config.Theme.ProgressBar, m.config.Theme.ProgressBarFill)
			bar.WriteString(gradientBar)
		}

		if empty > 0 {
			bar.WriteString(progressBgStyle.Render(strings.Repeat(ProgressGradientEmpty, empty)))
		}

	case "rainbow":
		// Rainbow gradient style with animation
		if filled > 0 {
			rainbowBar := m.createRainbowBar(filled, m.rainbowOffset)
			bar.WriteString(rainbowBar)
		}

		if empty > 0 {
			bar.WriteString(progressBgStyle.Render(strings.Repeat(ProgressGradientEmpty, empty)))
		}

	default:
		// Default to gradient
		if filled > 0 {
			gradientBar := m.createGradientBar(filled, m.config.Theme.ProgressBar, m.config.Theme.ProgressBarFill)
			bar.WriteString(gradientBar)
		}

		if empty > 0 {
			bar.WriteString(progressBgStyle.Render(strings.Repeat(ProgressGradientEmpty, empty)))
		}
	}

	return bar.String()
}

// createGradientBar creates a gradient effect between two colors.
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

// createRainbowBar creates an animated rainbow gradient progress bar.
func (m *Model) createRainbowBar(width int, t int) string {
	if width <= 0 {
		return ""
	}

	result := ""

	// Fixed gradient length - one full rainbow cycle every 120 characters
	gradientLength := 120.0
	// Create rainbow gradient with offset for animation
	for i := 0; i < width; i++ {
		// Calculate position in the gradient cycle
		position := float64(i) / gradientLength
		// Add time offset to create animation effect
		hue := math.Mod(position*360+float64(t), 360)
		// Convert HSL to RGB with softer, more pleasant colors
		// Lower saturation (0.6) and higher lightness (0.7) for pastel colors
		r, g, b := hslToRGB(hue, 0.6, 0.7)

		// Create color string in hex format
		color := fmt.Sprintf("#%02x%02x%02x", r, g, b)

		// Apply color to character
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
		result += style.Render("━")
	}

	return result
}

// hslToRGB converts HSL color values to RGB.
func hslToRGB(h, s, l float64) (uint8, uint8, uint8) {
	h /= 360.0

	var r, g, b float64

	if s == 0 {
		r, g, b = l, l, l
	} else {
		var hue2rgb = func(p, q, t float64) float64 {
			if t < 0 {
				t++
			}
			if t > 1 {
				t--
			}
			if t < 1.0/6.0 {
				return p + (q-p)*6*t
			}
			if t < 1.0/2.0 {
				return q
			}
			if t < 2.0/3.0 {
				return p + (q-p)*(2.0/3.0-t)*6
			}
			return p
		}

		var q float64
		if l < 0.5 {
			q = l * (1 + s)
		} else {
			q = l + s - l*s
		}

		p := 2*l - q

		r = hue2rgb(p, q, h+1.0/3.0)
		g = hue2rgb(p, q, h)
		b = hue2rgb(p, q, h-1.0/3.0)
	}

	return uint8(r * 255), uint8(g * 255), uint8(b * 255)
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
	isHomeView := m.state == HomeView
	hasMultipleSections := len(m.sections) > 1
	hint := m.shortcutFormatter.FormatHints(m.shortcutFormatter.GetPlayerHints(isHomeView, hasMultipleSections))
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
