package tui

import "github.com/charmbracelet/lipgloss"

const (
	cyan     = lipgloss.Color("#79c3ee")
	black    = lipgloss.Color("#101419")
	green    = lipgloss.Color("#78dba9")
	hotPink  = lipgloss.Color("#FF06B7")
	darkGray = lipgloss.Color("#767676")
	red      = lipgloss.Color("#e05f65")
)

var (
	selectedStyle = lipgloss.NewStyle().
			Background(cyan).
			Foreground(black)

	unSelectedStyle = lipgloss.NewStyle().UnsetBackground().UnsetForeground()
	inputStyle      = lipgloss.NewStyle().Foreground(cyan)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(hotPink).
			Padding(2, 2, 2, 4).
			Align(lipgloss.Left).
			Width(80)

	buttonStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(green)).Bold(true)
	buttonFocusedStyle = lipgloss.NewStyle().UnsetBold().Background(lipgloss.Color(green)).Foreground(lipgloss.Color(black)).Bold(true)
	errorStyle         = lipgloss.NewStyle().Foreground(red).Bold(true)
	helpStyle          = lipgloss.NewStyle().Foreground(darkGray)
)
