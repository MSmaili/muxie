package core

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/MSmaili/hetki/internal/terminal"
)

func TestSearchBarRendersModeAppropriateContent(t *testing.T) {
	prompt := lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("2"))
	query := lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	got := RenderSearchBar(SearchBarProps{
		Width: 32, Prompt: " / ", Right: "3/3", Style: query, PromptStyle: prompt,
	})
	if !strings.Contains(got, prompt.Render(" / ")) || terminal.Width(got) != 32 {
		t.Fatalf("search shortcut header = %q", got)
	}

	active := RenderSearchBar(SearchBarProps{
		Width: 20, Prompt: " / ", Filter: "dev", Active: true, Style: query, PromptStyle: prompt,
	})
	if !strings.Contains(active, query.Render(" dev█")) {
		t.Fatalf("active query used the wrong styling: %q", active)
	}
	if plain := strings.TrimRight(terminal.Sanitize(active), " "); plain != " /  dev█" {
		t.Fatalf("active search header = %q", plain)
	}

	hintStyle := lipgloss.NewStyle().Italic(true)
	centered := RenderSearchBar(SearchBarProps{
		Width: 32, Hint: "press key to jump", Right: "3/3", HintStyle: hintStyle,
	})
	if !strings.Contains(centered, hintStyle.Render("press key to jump")) {
		t.Fatalf("centered hint did not use its style: %q", centered)
	}
	plain := terminal.Sanitize(centered)
	if strings.HasPrefix(strings.TrimLeft(plain, " "), "/") || strings.Index(plain, "press key to jump") != (32-len("press key to jump"))/2 {
		t.Fatalf("jump header = %q", plain)
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
