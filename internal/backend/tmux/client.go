package tmux

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Client interface {
	Run(context.Context, ...string) (string, error)
	Execute(context.Context, Action) error
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

func (c *client) Run(ctx context.Context, args ...string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, c.bin, args...)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	// Raw output: #{q:...} parsers need exact record boundaries.
	output := out.String()

	if err != nil {
		return output, commandError(fmt.Sprintf("tmux %v", args), err, ctx.Err(), stderr.String())
	}

	return output, nil
}

func (c *client) Execute(ctx context.Context, action Action) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, c.bin, action.Args()...)
	switch action.(type) {
	case SwitchClient, AttachSession:
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return commandError(fmt.Sprintf("tmux %v", action.Args()), err, ctx.Err(), stderr.String())
	}
	return nil
}

func commandError(operation string, execErr, ctxErr error, stderr string) error {
	if ctxErr != nil {
		execErr = errors.Join(ctxErr, execErr)
	}
	if detail := strings.TrimSpace(stderr); detail != "" {
		return fmt.Errorf("%s failed: %w (%s)", operation, execErr, detail)
	}
	return fmt.Errorf("%s failed: %w", operation, execErr)
}
