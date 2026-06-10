package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme Colors - Project Visual Identity
const (
	ColorObsidianNight = lipgloss.Color("#0B0E14") // Background
	ColorVoidGrey      = lipgloss.Color("#161B22") // Surface
	ColorNeonPulse     = lipgloss.Color("#00F5FF") // Accent
	ColorGitGreen      = lipgloss.Color("#2EA043") // Status
	ColorSyntaxGrey    = lipgloss.Color("#8B949E") // Typography
)

// Shared Styles
var (
	AppStyle = lipgloss.NewStyle().
			Foreground(ColorSyntaxGrey).
			Background(ColorObsidianNight)

	// A clean, solid background container without rounded borders,
	// spanning across the UI, mimicking a modern IDE's surface area.
	TopSectionStyle = lipgloss.NewStyle().
			Background(ColorVoidGrey)

	// Column separator within the top section
	ColumnBorderStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("#30363D")). // Subtle border color
				BorderLeft(true)

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorNeonPulse).
			MarginBottom(1) // Add breathing room

	ItemStyle = lipgloss.NewStyle().
			Foreground(ColorSyntaxGrey)

	HighlightStyle = lipgloss.NewStyle().
			Foreground(ColorNeonPulse)

	// Output area feels like an embedded terminal pane without heavy borders
	OutputBoxStyle = lipgloss.NewStyle().
			Background(ColorObsidianNight)

	// Input styles
	PromptStyle    = lipgloss.NewStyle().Foreground(ColorNeonPulse).Bold(true)
	PathStyle      = lipgloss.NewStyle().Foreground(ColorSyntaxGrey).Italic(true)
	InputLineStyle = lipgloss.NewStyle().
			Background(ColorVoidGrey).
			Padding(0, 1)

	// Header/Status Bar
	HeaderStyle = lipgloss.NewStyle().
			Background(ColorVoidGrey).
			Foreground(ColorSyntaxGrey).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(lipgloss.Color("#30363D")).
			Padding(0, 1)

	LogoStyle = lipgloss.NewStyle().
			Foreground(ColorNeonPulse).
			Bold(true)
)
