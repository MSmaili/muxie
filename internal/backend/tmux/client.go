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
	// Raw output: #{q:...} parsers need exact record boundaries.
	output := out.String()

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
