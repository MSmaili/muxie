package core

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/MSmaili/hetki/internal/terminal"
)

func TestShortenPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		maxW int
	}{
		{name: "fits unchanged", path: "~/code", maxW: 20},
		{name: "elides middle keeping tail", path: "~/if_it_is_long/the-end", maxW: 16},
		{name: "very tight width", path: "~/a/b/c/d/e", maxW: 4},
		{name: "tail longer than width falls back", path: "~/short/averylongfinalsegment", maxW: 10},
		{name: "CJK emoji and combining marks", path: "~/项目/👨‍👩‍👧/éditor", maxW: 12},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortenPath(tt.path, tt.maxW)
			if w := lipgloss.Width(got); w > tt.maxW {
				t.Fatalf("shortenPath(%q, %d) = %q, width %d exceeds max", tt.path, tt.maxW, got, w)
			}
		})
	}
}

func TestShortenPathKeepsFinalSegmentWhenItFits(t *testing.T) {
	got := shortenPath("~/if_it_is_long/the-end", 16)
	if !strings.HasSuffix(got, "/the-end") {
		t.Fatalf("expected the final segment to be preserved, got %q", got)
	}
	if !strings.Contains(got, "...") {
		t.Fatalf("expected an ellipsis in the middle, got %q", got)
	}
}

func TestShortenPathPreservesRootAndAsManyFinalComponentsAsFit(t *testing.T) {
	got := shortenPath("~/dev/a-very-long-component/internal/tui", 22)
	if got != "~/.../internal/tui" {
		t.Fatalf("expected rooted component elision, got %q", got)
	}
}

func TestShortenPathZeroWidth(t *testing.T) {
	if got := shortenPath("~/code", 0); got != "" {
		t.Fatalf("expected empty string for zero width, got %q", got)
	}
}

func TestRenderTreeShowsJumpLabelBesideTheRow(t *testing.T) {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	line := RenderTree(TreeProps{
		Width: 40, Styles: TreeStyles{JumpLabel: labelStyle},
		Rows: []TreeRowProps{{Primary: "editor", JumpLabel: "aa"}},
	})[0]
	if plain := terminal.Sanitize(line); plain != "  aa editor" {
		t.Fatalf("jump label was not placed beside its row: %q", plain)
	}
	if _, noColor := defaultTheme().jumpLabel.GetBackground().(lipgloss.NoColor); !noColor {
		t.Fatal("jump labels must not form a continuous background column")
	}
	if width := terminal.Width(labelStyle.Render(displayJumpLabel("a"))); width != 1 {
		t.Fatalf("single-character jump label width = %d, want one cell", width)
	}
}

func TestRenderTreeUsesFullWidthSelectionAndCompactIndicators(t *testing.T) {
	selected := lipgloss.NewStyle().Background(lipgloss.Color("1"))
	secondarySelected := lipgloss.NewStyle().Background(lipgloss.Color("1")).Italic(true)
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	styles := TreeStyles{
		Secondary:         lipgloss.NewStyle().Italic(true),
		SecondarySelected: secondarySelected,
		SelectedRow:       selected,
		JumpLabel:         labelStyle,
	}
	got := RenderTree(TreeProps{Width: 24, Compact: true, Styles: styles, Rows: []TreeRowProps{{
		Primary: "editor", Secondary: "~/code", JumpLabel: "a", Active: true, Selected: true,
	}}})[0]
	want := selected.Width(24).Render("│ a editor  " + secondarySelected.Render("~/code"))
	if got != want {
		t.Fatalf("selected row was not styled as one full-width line:\n got %q\nwant %q", got, want)
	}
	if plain := strings.TrimRight(terminal.Sanitize(got), " "); plain != "│ a editor  ~/code" {
		t.Fatalf("selected row has excess markers or spacing: %q", plain)
	}
}

func TestRenderTreeMakesOnlyTheActiveFlatRowBold(t *testing.T) {
	styles := TreeStyles{
		Row:       lipgloss.NewStyle().Foreground(lipgloss.Color("1")),
		ActiveRow: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2")),
	}
	lines := RenderTree(TreeProps{Width: 24, Compact: true, Styles: styles, Rows: []TreeRowProps{
		{Primary: "inactive"},
		{Primary: "active", Active: true},
	}})
	if lines[0] != styles.Row.Render("  inactive") {
		t.Fatalf("inactive flat row used the wrong typography: %q", lines[0])
	}
	if lines[1] != styles.ActiveRow.Render("│ active") {
		t.Fatalf("active flat row used the wrong typography: %q", lines[1])
	}
}

func TestRenderTreeAlignsSecondaryColumnsAndTruncatesBothSides(t *testing.T) {
	lines := RenderTree(TreeProps{Width: 48, Rows: []TreeRowProps{
		{Primary: "dev > editor", Secondary: "~/code"},
		{Primary: "production > logs", Secondary: "/var/log"},
	}})
	if strings.Index(lines[0], "~/code") != strings.Index(lines[1], "/var/log") {
		t.Fatalf("secondary columns are not aligned: %#v", lines)
	}

	compact := RenderTree(TreeProps{Width: 40, Compact: true, Rows: []TreeRowProps{
		{Primary: "dev > editor", Secondary: "~/code"},
		{Primary: "production > logs", Secondary: "/var/log"},
	}})
	if !strings.Contains(compact[0], "~/code") || strings.Index(compact[0], "~/code") != strings.Index(compact[1], "/var/log") {
		t.Fatalf("compact flat rows lost aligned paths: %#v", compact)
	}

	line := RenderTree(TreeProps{Width: 24, Rows: []TreeRowProps{{
		Primary: "a-very-long-session > a-very-long-window", Secondary: "~/a/very/long/final-path",
	}}})[0]
	if !strings.Contains(line, "...") || !strings.Contains(line, "path") {
		t.Fatalf("expected independent middle path elision, got %q", line)
	}
	if width := lipgloss.Width(line); width > 24 {
		t.Fatalf("rendered width %d exceeds 24: %q", width, line)
	}
}

func TestRenderTreeSanitizesAndFitsNarrowWidths(t *testing.T) {
	row := TreeRowProps{Primary: "\x1b]0;owned\a中👨‍👩‍👧é\nnext", Secondary: "~/bad\tpath\x1b[2J", JumpLabel: "\x1b[31ma\n"}
	for width := 0; width <= 12; width++ {
		line := RenderTree(TreeProps{Width: width, Rows: []TreeRowProps{row}})[0]
		if strings.ContainsAny(line, "\x1b\n\r\t") {
			t.Fatalf("width %d left terminal controls in %q", width, line)
		}
		if actual := lipgloss.Width(line); actual > width {
			t.Fatalf("width %d rendered %q at width %d", width, line, actual)
		}
	}
}
