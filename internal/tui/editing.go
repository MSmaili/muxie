package tui

import (
	"strings"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func editEndOfLine(value string, keys KeyMap, mode KeyMode, msg tea.KeyPressMsg) (string, bool) {
	switch {
	case keys.Matches(mode, ActionBackspace, msg):
		runes := []rune(value)
		if len(runes) > 0 {
			value = string(runes[:len(runes)-1])
		}
		return value, true
	case keys.Matches(mode, ActionDeleteWord, msg):
		return deleteLastWord(value), true
	case keys.Matches(mode, ActionDeleteToStart, msg):
		return "", true
	case msg.Text != "":
		return value + msg.Text, true
	default:
		return value, false
	}
}

func singleLinePaste(content string) string {
	content = ansi.Strip(strings.TrimRight(content, "\r\n"))
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\r' || r == '\n' || r == '\t':
			return ' '
		case r == utf8.RuneError || unicode.IsControl(r):
			return -1
		default:
			return r
		}
	}, content)
}

func deleteLastWord(value string) string {
	runes := []rune(value)
	end := len(runes)
	for end > 0 && (runes[end-1] == ' ' || runes[end-1] == '\t') {
		end--
	}
	for end > 0 && runes[end-1] != ' ' && runes[end-1] != '\t' {
		end--
	}
	return string(runes[:end])
}
