package terminal

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

// Sanitize removes terminal sequences and makes control characters visible.
func Sanitize(s string) string {
	s = ansi.Strip(s)
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if unicode.IsControl(r) {
				if r <= 0xff {
					fmt.Fprintf(&b, `\x%02x`, r)
				} else {
					fmt.Fprintf(&b, `\u%04x`, r)
				}
				continue
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}

func Width(s string) int { return ansi.StringWidth(s) }

func Cut(s string, left, right int) string { return ansi.Cut(s, left, right) }

func Truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if width <= 3 {
		return ansi.Truncate(s, width, "")
	}
	return ansi.Truncate(s, width, "...")
}
