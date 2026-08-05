package core

import "charm.land/lipgloss/v2"

// selectionBg is the highlighted row's background; content drawn on a selected
// row must use it to stay readable.
var selectionBg = lipgloss.Color("60")

type theme struct {
	meta               lipgloss.Style
	searchBox          lipgloss.Style
	row                lipgloss.Style
	sessionRow         lipgloss.Style
	windowRow          lipgloss.Style
	windowPath         lipgloss.Style
	windowPathSelected lipgloss.Style
	activeRow          lipgloss.Style
	selectedRow        lipgloss.Style
	selectedHint       lipgloss.Style
	rail               lipgloss.Style
	sectionLine        lipgloss.Style
	helpOverlay        lipgloss.Style
	appBorder          lipgloss.Style
	modal              lipgloss.Style
	modalTitle         lipgloss.Style
	modalHint          lipgloss.Style
}

func defaultTheme() theme {
	return theme{
		meta:       lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		searchBox:  lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		row:        lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		sessionRow: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("224")),
		windowRow:  lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		windowPath: lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true),
		// Path on a selected row: light foreground on the selection background
		// so it stays readable against the highlight.
		windowPathSelected: lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Background(selectionBg).Italic(true),
		activeRow:          lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("181")),
		selectedRow:        lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(selectionBg),
		selectedHint:       lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("181")),
		rail:               lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		sectionLine:        lipgloss.NewStyle().Foreground(lipgloss.Color("237")),
		helpOverlay: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1),
		appBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("110")).
			Padding(0, 2),
		modal: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("110")).
			Padding(0, 1),
		modalTitle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("110")),
		modalHint:  lipgloss.NewStyle().Foreground(lipgloss.Color("246")),
	}
}
