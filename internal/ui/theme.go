package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/haryoiro/yutemal/internal/structures"
)

// ThemeManager manages UI styles based on the configured theme
type ThemeManager struct {
	theme structures.Theme

	// Cached styles
	baseStyle         lipgloss.Style
	selectedStyle     lipgloss.Style
	playingStyle      lipgloss.Style
	borderStyle       lipgloss.Style
	progressStyle     lipgloss.Style
	progressFillStyle lipgloss.Style
	titleStyle        lipgloss.Style
	subtitleStyle     lipgloss.Style
	helpStyle         lipgloss.Style
}

// NewThemeManager creates a new theme manager with the given theme
func NewThemeManager(theme structures.Theme) *ThemeManager {
	tm := &ThemeManager{theme: theme}
	tm.initStyles()
	return tm
}

// initStyles initializes all the cached styles
func (tm *ThemeManager) initStyles() {
	// Base style with foreground only (no background to avoid partial coloring)
	tm.baseStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(tm.theme.Foreground))

	// Selected item style (using only foreground color and bold)
	tm.selectedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(tm.theme.Selected)).
		Bold(true)

	// Playing item style
	tm.playingStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(tm.theme.Playing)).
		Bold(true)

	// Border style
	tm.borderStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(tm.theme.Border))

	// Progress bar styles
	tm.progressStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(tm.theme.ProgressBar))

	tm.progressFillStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(tm.theme.ProgressBarFill))

	// Text styles
	tm.titleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(tm.theme.Foreground)).
		Bold(true)

	tm.subtitleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(tm.theme.Foreground)).
		Faint(true)

	tm.helpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(tm.theme.Foreground)).
		Faint(true).
		Italic(true)
}

// Update updates the theme and reinitializes styles
func (tm *ThemeManager) Update(theme structures.Theme) {
	tm.theme = theme
	tm.initStyles()
}

// Getters for various styles

func (tm *ThemeManager) BaseStyle() lipgloss.Style {
	return tm.baseStyle
}

func (tm *ThemeManager) SelectedStyle() lipgloss.Style {
	return tm.selectedStyle
}

func (tm *ThemeManager) PlayingStyle() lipgloss.Style {
	return tm.playingStyle
}

func (tm *ThemeManager) BorderStyle() lipgloss.Style {
	return tm.borderStyle
}

func (tm *ThemeManager) ProgressStyle() lipgloss.Style {
	return tm.progressStyle
}

func (tm *ThemeManager) ProgressFillStyle() lipgloss.Style {
	return tm.progressFillStyle
}

func (tm *ThemeManager) TitleStyle() lipgloss.Style {
	return tm.titleStyle
}

func (tm *ThemeManager) SubtitleStyle() lipgloss.Style {
	return tm.subtitleStyle
}

func (tm *ThemeManager) HelpStyle() lipgloss.Style {
	return tm.helpStyle
}

// Helper methods for common styling patterns

func (tm *ThemeManager) RenderTitle(text string) string {
	return tm.titleStyle.Render(text)
}

func (tm *ThemeManager) RenderSubtitle(text string) string {
	return tm.subtitleStyle.Render(text)
}

func (tm *ThemeManager) RenderSelected(text string) string {
	return tm.selectedStyle.Render(text)
}

func (tm *ThemeManager) RenderPlaying(text string) string {
	return tm.playingStyle.Render(text)
}

func (tm *ThemeManager) RenderHelp(text string) string {
	return tm.helpStyle.Render(text)
}

// GetDefaultTheme returns the default theme
func GetDefaultTheme() structures.Theme {
	return structures.Theme{
		Background:       "#1a1b26",  // Tokyo Night Storm background
		Foreground:       "#c0caf5",  // Tokyo Night foreground
		Selected:         "#7aa2f7",  // Tokyo Night blue
		Playing:          "#9ece6a",  // Tokyo Night green
		Border:           "#3b4261",  // Tokyo Night border
		ProgressBar:      "#565f89",  // Tokyo Night dark gray
		ProgressBarFill:  "#7aa2f7",  // Tokyo Night blue
		ProgressBarStyle: "gradient", // Default to gradient style
	}
}
