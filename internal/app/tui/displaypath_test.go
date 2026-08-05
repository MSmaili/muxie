package tui

import "testing"

func TestDisplayPath(t *testing.T) {
	home := "/Users/me"
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "collapses home to tilde", path: "/Users/me/code/app", want: "~/code/app"},
		{name: "home itself becomes tilde", path: "/Users/me", want: "~"},
		{name: "path outside home unchanged", path: "/etc/hosts", want: "/etc/hosts"},
		{name: "empty stays empty", path: "", want: ""},
		{name: "trims surrounding space", path: "  /Users/me/x  ", want: "~/x"},
		{name: "home prefix without separator not collapsed", path: "/Users/meextra", want: "/Users/meextra"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := displayPath(tt.path, home); got != tt.want {
				t.Fatalf("displayPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestDisplayPathNoHome(t *testing.T) {
	if got := displayPath("/Users/me/x", ""); got != "/Users/me/x" {
		t.Fatalf("with empty home, path should be unchanged, got %q", got)
	}
}
