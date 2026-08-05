package components

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
