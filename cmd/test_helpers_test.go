package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/MSmaili/hetki/internal/backend"
	"github.com/MSmaili/hetki/internal/logger"
	"github.com/fatih/color"
	"github.com/stretchr/testify/require"
)

type stubBackend struct {
	queryResult backend.StateResult
	queryErr    error
	dryRunLines []string
	applyErr    error

	applyCalls  int
	attachCalls int
	dryRunCalls int
	lastActions []backend.Action
	lastAttach  string
	lastSwitch  string
}

func (s *stubBackend) Name() string { return "stub" }

func (s *stubBackend) QueryState(context.Context) (backend.StateResult, error) {
	if s.queryErr != nil {
		return backend.StateResult{}, s.queryErr
	}
	return s.queryResult, nil
}

func (s *stubBackend) Apply(_ context.Context, actions []backend.Action) error {
	s.applyCalls++
	s.lastActions = append([]backend.Action(nil), actions...)
	return s.applyErr
}

func (s *stubBackend) DryRun(actions []backend.Action) ([]string, error) {
	s.dryRunCalls++
	s.lastActions = append([]backend.Action(nil), actions...)
	return append([]string(nil), s.dryRunLines...), nil
}

func (s *stubBackend) Attach(_ context.Context, session string) error {
	s.attachCalls++
	s.lastAttach = session
	return nil
}

func (s *stubBackend) Switch(_ context.Context, target string) error {
	s.lastSwitch = target
	return nil
}

func resetCommandGlobals() {
	listCmd.SetContext(context.Background())
	saveCmd.SetContext(context.Background())
	startCmd.SetContext(context.Background())
	dryRun = false
	force = false
	listSessions = false
	listWindows = false
	listPanes = false
	listFormat = "flat"
	listDelimiter = ":"
	listCurrent = false
	listMarker = ""
	savePath = ""
	saveName = ""
	saveAll = false
}

func withStubBackend(t *testing.T, stub backend.Backend) {
	t.Helper()
	previous := detectBackend
	detectBackend = func(name ...string) (backend.Backend, error) {
		return stub, nil
	}
	t.Cleanup(func() {
		detectBackend = previous
	})
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	previous := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	fn()
	require.NoError(t, w.Close())
	os.Stdout = previous

	data, err := io.ReadAll(r)
	require.NoError(t, err)
	require.NoError(t, r.Close())
	return string(data)
}

func captureLoggerOutput(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	previousOutput := logger.SetOutput(&buf)
	previousNoColor := color.NoColor
	color.NoColor = true
	defer func() {
		logger.SetOutput(previousOutput)
		color.NoColor = previousNoColor
	}()

	fn()
	return buf.String()
}

func writeWorkspaceFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}
