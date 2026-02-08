package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/MSmaili/hetki/internal/backend"
	"github.com/spf13/cobra"
)

var switchCmd = &cobra.Command{
	Use:   "switch [target]",
	Short: "Switch to a session, window, or pane",
	Long: `Switch to a tmux target (session, window, or pane).

The target can be passed as an argument or piped from stdin:
  hetki switch dev
  hetki switch dev:editor
  hetki switch dev:editor:0
  hetki list sessions -w | fzf | hetki switch`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSwitch,
}

func init() {
	rootCmd.AddCommand(switchCmd)
}

func runSwitch(cmd *cobra.Command, args []string) error {
	var raw string

	if len(args) > 0 {
		raw = args[0]
	} else {
		line, err := readStdinLine()
		if err != nil {
			return fmt.Errorf("no target provided\nUsage: hetki switch <target> or pipe from stdin")
		}
		raw = line
	}

	target := parseTarget(raw)
	if target == "" {
		return fmt.Errorf("empty target")
	}

	b, err := backend.Detect()
	if err != nil {
		return fmt.Errorf("failed to detect backend: %w", err)
	}

	if err := b.Switch(target); err != nil {
		return fmt.Errorf("switch to %q: %w", target, err)
	}
	return nil
}

func readStdinLine() (string, error) {
	info, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return "", fmt.Errorf("stdin is a terminal")
	}

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return scanner.Text(), nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("empty stdin")
}

func parseTarget(raw string) string {
	s := strings.TrimSpace(raw)
	s = stripMarkerPrefix(s)
	s = listToTmuxFormat(s)
	return s
}

func stripMarkerPrefix(s string) string {
	i := strings.IndexFunc(s, func(r rune) bool {
		return r == ':' || unicode.IsLetter(r) || unicode.IsDigit(r)
	})
	if i > 0 {
		s = s[i:]
	}
	return strings.TrimSpace(s)
}

func listToTmuxFormat(s string) string {
	parts := strings.Split(s, ":")
	if len(parts) == 3 {
		return parts[0] + ":" + parts[1] + "." + parts[2]
	}
	return s
}
