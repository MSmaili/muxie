//go:build integration

package tui

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	backendtmux "github.com/MSmaili/hetki/internal/backend/tmux"
	"github.com/MSmaili/hetki/internal/tui/core"
	"github.com/MSmaili/hetki/internal/tui/list"
	"github.com/stretchr/testify/require"
)

const ptyHelperEnv = "HETKI_TUI_PTY_HELPER"

type ptyDriver struct {
	navigate func(context.Context, core.BackendTarget) error
}

func (ptyDriver) Load(context.Context) (list.Snapshot, error) {
	return list.Snapshot{Items: []list.Item{{
		ID: "session:dev", Primary: "dev",
		SearchFields: []list.SearchField{{Tier: list.SearchPrimary, Text: "dev"}},
	}}}, nil
}

func (ptyDriver) Execute(_ context.Context, request core.ActionRequest) (core.ActionResult, error) {
	if request.ActionID != core.ActionOpen || request.ItemID != "session:dev" {
		return core.ActionResult{}, fmt.Errorf("unexpected jump request: %+v", request)
	}
	return core.ActionResult{Navigation: "dev"}, nil
}

func (d ptyDriver) Navigate(ctx context.Context, target core.BackendTarget) error {
	return d.navigate(ctx, target)
}

func TestTerminalRestoredBeforeNavigation(t *testing.T) {
	if os.Getenv(ptyHelperEnv) == "1" {
		b, err := backendtmux.NewBackend()
		require.NoError(t, err)
		driver := ptyDriver{navigate: func(ctx context.Context, target core.BackendTarget) error {
			if err := b.Switch(ctx, string(target)); err != nil {
				return err
			}
			fmt.Println("RECORDED")
			return nil
		}}
		require.NoError(t, (Service{Driver: driver, RunUI: core.RunWithKeyMap}).Run(context.Background()))
		return
	}

	script, err := exec.LookPath("script")
	if err != nil {
		t.Skip("script is required for PTY integration coverage")
	}
	testBinary, err := os.Executable()
	require.NoError(t, err)

	var args []string
	if runtime.GOOS == "darwin" {
		args = []string{"-q", "/dev/null", testBinary, "-test.run=^TestTerminalRestoredBeforeNavigation$"}
	} else {
		command := fmt.Sprintf("%q -test.run=^TestTerminalRestoredBeforeNavigation$", testBinary)
		args = []string{"-q", "-e", "-c", command, "/dev/null"}
	}
	fakeBin := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(fakeBin, "tmux"), []byte("#!/bin/sh\nprintf 'NAVIGATION\\n'\n"), 0755))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, script, args...)
	cmd.Env = append(os.Environ(), ptyHelperEnv+"=1", "TERM=xterm-256color", "PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	input, inputWriter, err := os.Pipe()
	require.NoError(t, err)
	defer input.Close()
	cmd.Stdin = input
	go func() {
		time.Sleep(500 * time.Millisecond)
		_, _ = inputWriter.Write([]byte("a"))
		_ = inputWriter.Close()
	}()
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	require.NoError(t, cmd.Run(), output.String())

	bytesOut := output.Bytes()
	restored := bytes.Index(bytesOut, []byte("\x1b[?1049l"))
	navigation := bytes.Index(bytesOut, []byte("NAVIGATION"))
	recorded := bytes.Index(bytesOut, []byte("RECORDED"))
	require.NotEqual(t, -1, restored, "alternate screen was not restored: %q", output.String())
	require.Greater(t, navigation, restored, "navigation ran before terminal restoration: %q", output.String())
	require.Greater(t, recorded, navigation, "recording ran before navigation succeeded: %q", output.String())
}
