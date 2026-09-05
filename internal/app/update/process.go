package update

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

type boundedBuffer struct {
	bytes.Buffer
	max int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.max-b.Len() {
		return 0, fmt.Errorf("command output exceeds %d bytes", b.max)
	}
	return b.Buffer.Write(p)
}

func commandOutput(ctx context.Context, timeout time.Duration, maxBytes int, name string, args ...string) ([]byte, error) {
	return commandOutputEnv(ctx, timeout, maxBytes, nil, name, args...)
}

func commandOutputEnv(ctx context.Context, timeout time.Duration, maxBytes int, env []string, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	defer killCommandGroup(cmd)
	if env != nil {
		cmd.Env = env
	}
	cmd.WaitDelay = 2 * time.Second
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	output := &boundedBuffer{max: maxBytes}
	cmd.Stdout = output
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, err
	}
	return output.Bytes(), nil
}

func killCommandGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
