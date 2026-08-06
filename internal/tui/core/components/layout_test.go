package components

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/MSmaili/hetki/internal/terminal"
)

func TestHeadersAndModalsSanitizeAndFit(t *testing.T) {
	unsafe := "中👨‍👩‍👧é\x1b]0;owned\a\nnext"
	border := lipgloss.NewStyle().Border(lipgloss.RoundedBorder())

	for width := 0; width <= 20; width++ {
		rendered := []string{
			RenderSearchBar(SearchBarProps{Width: width, Filter: unsafe, Right: unsafe}),
			RenderInputModal(InputModalProps{LineWidth: width, Title: "INPUT", Prompt: unsafe, Value: unsafe, ModalStyle: border}),
			RenderConfirmModal(ConfirmModalProps{LineWidth: width, Title: "CONFIRM", Body: unsafe, ModalStyle: border}),
			RenderHelpOverlay(HelpOverlayProps{LineWidth: width, Title: "HELP", Hint: unsafe, Sections: []HelpSection{{Title: "KEYS", Entries: []HelpEntry{{Keys: "?", Desc: unsafe}}}}, OverlayStyle: border}),
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
