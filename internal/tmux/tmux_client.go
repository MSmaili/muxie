package tmux

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/MSmaili/tmx/internal/domain"
)

type TmuxClient struct {
	bin string
}

func New() (*TmuxClient, error) {
	bin, err := exec.LookPath("tmux")
	if err != nil {
		return nil, fmt.Errorf("tmux not found in PATH")
	}
	return &TmuxClient{bin: bin}, nil
}

func (c *TmuxClient) run(args ...string) (string, error) {
	cmd := exec.Command(c.bin, args...)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("tmux %v failed: %v (%s)", args, err, stderr.String())
	}

	return strings.TrimSpace(out.String()), nil
}

func (c *TmuxClient) ListSessions() (map[string][]domain.Window, error) {
	out, err := c.run("list-sessions", "-F", "#{session_name}")
	if err != nil {
		return nil, err
	}

	sessions := strings.Split(out, "\n")
	result := make(map[string][]domain.Window)

	for _, session := range sessions {
		if session == "" {
			continue
		}
		windows, err := c.ListWindows(session)
		if err != nil {
			return nil, fmt.Errorf("list windows for session %s: %w", session, err)
		}

		result[session] = windows
	}

	return result, nil
}

func (c *TmuxClient) ListWindows(session string) ([]domain.Window, error) {
	out, err := c.run(
		"list-windows",
		"-t", session,
		"-F", "#{window_name}|#{pane_current_path}|#{window_index}|#{window_layout}",
	)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out, "\n")
	windows := make([]domain.Window, 0, len(lines))

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) != 4 {
			continue
		}

		// idx, err := strconv.Atoi(parts[2])
		// if err != nil {
		// 	return nil, err
		// }

		windows = append(windows, domain.Window{
			Name: parts[0],
			Path: parts[1],
			// Index:  &idx,
			Layout: parts[3],
		})
	}

	return windows, nil
}

func (c *TmuxClient) HasSession(name string) bool {
	_, err := c.run("has-session", "-t", name)
	return err == nil
}

func (c *TmuxClient) CreateSession(name string, opts *domain.Window) error {
	args := []string{"new-session", "-d", "-s", name}

	if opts == nil {
		opts = &domain.Window{}
	}

	if opts.Path != "" {
		args = append(args, "-c", opts.Path)
	}

	if opts.Name != "" {
		args = append(args, "-n", opts.Name)
	}

	if opts.Command != "" {
		args = append(args, opts.Command)
	}

	_, err := c.run(args...)
	return err
}

func (c *TmuxClient) CreateWindow(session string, name string, opts domain.Window) error {
	target := fmt.Sprintf("%s:%d", session, opts.Index)

	args := []string{"new-window", "-t", target, "-n", name, "-d"}

	if opts.Path != "" {
		args = append(args, "-c", opts.Path)
	}

	if opts.Command != "" {
		args = append(args, opts.Command)
	}

	_, err := c.run(args...)
	return err
}

func (c *TmuxClient) SetLayout(session string, window string, layout string) error {
	_, err := c.run("select-layout", "-t", fmt.Sprintf("%s:%s", session, window), layout)
	return err
}

func (c *TmuxClient) Attach(session string) error {
	cmd := exec.Command(c.bin, "attach-session", "-t", session)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (c *TmuxClient) KillSession(name string) error {
	_, err := c.run("kill-session", "-t", name)
	return err
}

func (c *TmuxClient) KillWindow(session, window string) error {
	target := fmt.Sprintf("%s:%s", session, window)
	_, err := c.run("kill-window", "-t", target)
	return err
}

func (c *TmuxClient) BaseWindowIndex() (int, error) {
	out, err := c.run("show-options", "-g", "base-index")
	if err != nil {
		return 0, err
	}

	parts := strings.Fields(out)
	if len(parts) != 2 {
		return 0, fmt.Errorf("unexpected base-index output: %s", out)
	}

	idx, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}

	return idx, nil
}
