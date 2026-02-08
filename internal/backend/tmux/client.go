package tmux

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Client interface {
	Run(args ...string) (string, error)
	Execute(action Action) error
	ExecuteBatch(actions []Action) error
}

type client struct {
	bin string
}

func New() (Client, error) {
	bin, err := exec.LookPath("tmux")
	if err != nil {
		return nil, fmt.Errorf("tmux not found in PATH")
	}
	return &client{bin: bin}, nil
}

func (c *client) Run(args ...string) (string, error) {
	cmd := exec.Command(c.bin, args...)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := strings.TrimSpace(out.String())

	if err != nil {
		return output, fmt.Errorf("tmux %v failed: %v (%s)", args, err, stderr.String())
	}

	return output, nil
}

func (c *client) Execute(action Action) error {
	cmd := exec.Command(c.bin, action.Args()...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if s := strings.TrimSpace(stderr.String()); s != "" {
			return fmt.Errorf("%s", s)
		}
		return err
	}
	return nil
}

func (c *client) ExecuteBatch(actions []Action) error {
	if len(actions) == 0 {
		return nil
	}

	err := c.executeSource(actions)
	if err != nil && c.isServerNotRunning(err) {
		if err := exec.Command(c.bin, actions[0].Args()...).Run(); err != nil {
			return fmt.Errorf("failed to start tmux: %w", err)
		}
		if len(actions) > 1 {
			return c.executeSource(actions[1:])
		}
		return nil
	}
	return err
}

func (c *client) executeSource(actions []Action) error {
	var script strings.Builder
	for _, action := range actions {
		script.WriteString(quoteArgs(action.Args()))
		script.WriteString("\n")
	}

	cmd := exec.Command(c.bin, "source", "-")
	cmd.Stdin = strings.NewReader(script.String())

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tmux source failed: %v\nstderr: %s\nscript:\n%s", err, stderr.String(), script.String())
	}
	return nil
}

func (c *client) isServerNotRunning(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no server running")
}

func quoteArgs(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		if strings.ContainsAny(arg, " \t\"'") {
			quoted[i] = fmt.Sprintf("%q", arg)
		} else {
			quoted[i] = arg
		}
	}
	return strings.Join(quoted, " ")
}
