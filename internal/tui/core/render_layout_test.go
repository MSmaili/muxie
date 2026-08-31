package core

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/MSmaili/hetki/internal/terminal"
)

func TestSearchBarShowsTheSlashShortcutPrompt(t *testing.T) {
	prompt := lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("2"))
	placeholder := lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	query := lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	got := RenderSearchBar(SearchBarProps{
		Width: 32, Right: "3/3", Style: query, PromptStyle: prompt, PlaceholderStyle: placeholder,
	})
	if !strings.Contains(got, prompt.Render(" / ")) {
		t.Fatalf("search shortcut is not rendered as a box: %q", got)
	}
	if !strings.Contains(got, placeholder.Render(" search destinations")) {
		t.Fatalf("search placeholder did not use its subdued style: %q", got)
	}
	if plain := terminal.Sanitize(got); plain != " /  search destinations      3/3" {
		t.Fatalf("search header = %q", plain)
	}

	active := RenderSearchBar(SearchBarProps{Width: 20, Filter: "dev", Active: true, Style: query, PromptStyle: prompt, PlaceholderStyle: placeholder})
	if !strings.Contains(active, query.Render(" dev_")) {
		t.Fatalf("query used placeholder styling: %q", active)
	}
	if plain := strings.TrimRight(terminal.Sanitize(active), " "); plain != " /  dev_" {
		t.Fatalf("active search header = %q", plain)
	}
}

func TestHeadersAndModalsSanitizeAndFit(t *testing.T) {
	unsafe := "中👨‍👩‍👧é\x1b]0;owned\a\nnext"
	border := lipgloss.NewStyle().Border(lipgloss.RoundedBorder())

	for width := 0; width <= 20; width++ {
		rendered := []string{
			RenderSearchBar(SearchBarProps{Width: width, Filter: unsafe, Right: unsafe}),
			RenderInputModal(InputModalProps{LineWidth: width, Title: "INPUT", Prompt: unsafe, Value: unsafe, ModalStyle: border}),
			RenderConfirmModal(ConfirmModalProps{LineWidth: width, Title: "CONFIRM", Body: unsafe, ModalStyle: border}),
			RenderMenu(MenuProps{
				LineWidth: width, Title: unsafe, ModalStyle: border,
				Entries: []MenuEntry{{Action: ActionOpen, Label: unsafe, Activation: 'o'}},
			}),
		}
		for _, output := range rendered {
			if strings.Contains(output, "owned") || strings.Contains(output, "\nnext") {
				t.Fatalf("width %d rendered unsafe text: %q", width, output)
			}
			for _, line := range strings.Split(output, "\n") {
				if actual := terminal.Width(line); actual > width {
					t.Fatalf("width %d rendered line width %d: %q", width, actual, line)
				}
			}
		}
	}
}
