package core

import "charm.land/lipgloss/v2"

var selectionBg = lipgloss.Color("60")

type theme struct {
	meta              lipgloss.Style
	searchBox         lipgloss.Style
	headerHint        lipgloss.Style
	row               lipgloss.Style
	rootRow           lipgloss.Style
	childRow          lipgloss.Style
	secondary         lipgloss.Style
	secondarySelected lipgloss.Style
	activeRow         lipgloss.Style
	selectedRow       lipgloss.Style
	searchPrompt      lipgloss.Style
	jumpLabel         lipgloss.Style
	rail              lipgloss.Style
	sectionLine       lipgloss.Style
	appBorder         lipgloss.Style
	modal             lipgloss.Style
	modalTitle        lipgloss.Style
	modalHint         lipgloss.Style
}

func defaultTheme() theme {
	return theme{
		meta:       lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		searchBox:  lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		headerHint: lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true),
		row:        lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		rootRow:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("224")),
		childRow:   lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		secondary:  lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true),
		secondarySelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).Background(selectionBg).Italic(true),
		activeRow:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("224")),
		selectedRow: lipgloss.NewStyle().Background(selectionBg),
		searchPrompt: lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color("235")).Background(lipgloss.Color("181")),
		jumpLabel:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("110")),
		rail:        lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		sectionLine: lipgloss.NewStyle().Foreground(lipgloss.Color("237")),
		appBorder:   lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("110")).Padding(0, 2),
		modal:       lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("110")).Padding(0, 1),
		modalTitle:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("110")),
		modalHint:   lipgloss.NewStyle().Foreground(lipgloss.Color("246")),
	}
}
