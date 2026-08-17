package core

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
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

func TestShortenPathZeroWidth(t *testing.T) {
	if got := shortenPath("~/code", 0); got != "" {
		t.Fatalf("expected empty string for zero width, got %q", got)
	}
}

func TestRenderTreeShowsJumpLabelBesideTheRow(t *testing.T) {
	line := RenderTree(TreeProps{Width: 40, Rows: []TreeRowProps{{Primary: "editor", JumpLabel: "aa"}}})[0]
	if !strings.Contains(line, "aa") || !strings.Contains(line, "editor") {
		t.Fatalf("jump label was not rendered beside row: %q", line)
	}
}

func TestRenderTreeSanitizesAndFitsNarrowWidths(t *testing.T) {
	row := TreeRowProps{Primary: "\x1b]0;owned\a中👨‍👩‍👧é\nnext", JumpLabel: "\x1b[31ma\n"}
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
