package tui

import "charm.land/lipgloss/v2"

// Semantic colors are shared by flat rows, tree rows, headers, and overlays.
const (
	colorText             = "250"
	colorMuted            = "245"
	colorSecondary        = "240"
	colorEmphasis         = "252"
	colorActive           = "224"
	colorAccent           = "110"
	colorSelection        = "60"
	colorPromptText       = "235"
	colorPromptBackground = "181"
	colorDivider          = "237"
)

type theme struct {
	meta              lipgloss.Style
	searchBox         lipgloss.Style
	headerHint        lipgloss.Style
	row               lipgloss.Style
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
	text := lipgloss.NewStyle().Foreground(lipgloss.Color(colorText))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
	secondary := lipgloss.NewStyle().Foreground(lipgloss.Color(colorSecondary))
	emphasis := lipgloss.NewStyle().Foreground(lipgloss.Color(colorEmphasis))
	accent := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorAccent))
	selection := lipgloss.NewStyle().Background(lipgloss.Color(colorSelection))
	border := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(colorAccent))
	return theme{
		meta:              muted,
		searchBox:         emphasis,
		headerHint:        muted.Italic(true),
		row:               text,
		secondary:         secondary.Italic(true),
		secondarySelected: emphasis.Inherit(selection).Italic(true),
		activeRow:         lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorActive)),
		selectedRow:       selection,
		searchPrompt:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorPromptText)).Background(lipgloss.Color(colorPromptBackground)),
		jumpLabel:         accent,
		rail:              secondary,
		sectionLine:       lipgloss.NewStyle().Foreground(lipgloss.Color(colorDivider)),
		appBorder:         border.Padding(0, 2),
		modal:             border.Padding(0, 1),
		modalTitle:        accent,
		modalHint:         muted,
	}
}
