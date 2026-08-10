package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

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
		line, err := readStdinLine(cmd.Context())
		if err != nil {
			return fmt.Errorf("no target provided\nUsage: hetki switch <target> or pipe from stdin: %w", err)
		}
		raw = line
	}

	target := parseTarget(raw)
	if target == "" {
		return fmt.Errorf("empty target")
	}

	b, err := detectBackend()
	if err != nil {
		return fmt.Errorf("failed to detect backend: %w", err)
	}

	if err := b.Switch(cmd.Context(), target); err != nil {
		return fmt.Errorf("switch to %q: %w", target, err)
	}
	return nil
}

func readStdinLine(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	info, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return "", fmt.Errorf("stdin is a terminal")
	}

	deadlineSet := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		_ = os.Stdin.SetReadDeadline(time.Now())
		close(deadlineSet)
	})
	defer func() {
		if !stop() {
			<-deadlineSet
		}
		_ = os.Stdin.SetReadDeadline(time.Time{})
	}()
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		return scanner.Text(), nil
	}
	if err := ctx.Err(); err != nil {
		return "", err
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
	space := strings.IndexFunc(s, unicode.IsSpace)
	if space > 0 && strings.IndexFunc(s[:space], func(r rune) bool {
		return r == '$' || unicode.IsLetter(r) || unicode.IsDigit(r)
	}) == -1 {
		return strings.TrimSpace(s[space:])
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
