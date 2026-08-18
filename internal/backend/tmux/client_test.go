package tmux

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func awaitTmuxChannel[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for test channel")
		var zero T
		return zero
	}
}

// MockClient for testing
type shellAction []string

func (a shellAction) Args() []string { return a }

type MockClient struct {
	RunFunc     func(context.Context, ...string) (string, error)
	ExecuteFunc func(context.Context, Action) error
}

func (m *MockClient) Run(ctx context.Context, args ...string) (string, error) {
	if m.RunFunc != nil {
		return m.RunFunc(ctx, args...)
	}
	return "", nil
}

func (m *MockClient) Execute(ctx context.Context, action Action) error {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, action)
	}
	return nil
}

func TestRunQuery(t *testing.T) {
	t.Setenv("TMUX", "")

	tests := []struct {
		name    string
		output  string
		runErr  error
		want    LoadStateResult
		wantErr bool
	}{
		{
			name:   "success",
			output: "0\n0\n$1|dev|@1|editor|0|layout-a|0|1|%1|0|1|~/code|vim|",
			want: LoadStateResult{
				Sessions: []Session{{ID: "$1", Name: "dev", Windows: []Window{{ID: "@1", Name: "editor", Index: 0, Path: "~/code", Layout: "layout-a", Active: true, Panes: []Pane{{ID: "%1", Index: 0, Path: "~/code", Command: "vim", Active: true}}}}}},
			},
		},
		{
			name:    "run error",
			runErr:  errors.New("tmux failed"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockClient{
				RunFunc: func(context.Context, ...string) (string, error) {
					return tt.output, tt.runErr
				},
			}

			got, err := RunQuery(context.Background(), mock, LoadStateQuery{})

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestRunQueryPreservesExecutionErrorWhenOutputIsMalformed(t *testing.T) {
	mock := &MockClient{RunFunc: func(context.Context, ...string) (string, error) {
		return "malformed", context.Canceled
	}}

	_, err := RunQuery(context.Background(), mock, LoadStateQuery{})

	require.ErrorIs(t, err, context.Canceled)
	assert.ErrorContains(t, err, "missing base indexes")
}

func TestClientRunCancelsStartedProcess(t *testing.T) {
	sh, err := exec.LookPath("sh")
	require.NoError(t, err)
	started := filepath.Join(t.TempDir(), "started")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := (&client{bin: sh}).Run(ctx, "-c", `echo started > "$1"; exec sleep 10`, "sh", started)
		done <- err
	}()
	require.Eventually(t, func() bool {
		_, err := os.Stat(started)
		return err == nil
	}, time.Second, 10*time.Millisecond)

	cancel()

	runErr := awaitTmuxChannel(t, done)
	require.ErrorIs(t, runErr, context.Canceled)
	var exitErr *exec.ExitError
	require.ErrorAs(t, runErr, &exitErr)
}

func TestClientNoninteractiveActionsDoNotWriteToTerminal(t *testing.T) {
	sh, err := exec.LookPath("sh")
	require.NoError(t, err)
	read, write, err := os.Pipe()
	require.NoError(t, err)
	originalStdout := os.Stdout
	os.Stdout = write
	t.Cleanup(func() { os.Stdout = originalStdout })

	require.NoError(t, (&client{bin: sh}).Execute(context.Background(), shellAction{"-c", "echo mutation-output"}))
	require.NoError(t, write.Close())
	output, err := io.ReadAll(read)
	require.NoError(t, err)
	assert.Empty(t, output)
}

func TestClientPreservesExitErrorsWithStderr(t *testing.T) {
	sh, err := exec.LookPath("sh")
	require.NoError(t, err)
	c := &client{bin: sh}

	_, runErr := c.Run(context.Background(), "-c", "echo denied >&2; exit 7")
	executeErr := c.Execute(context.Background(), shellAction{"-c", "echo denied >&2; exit 7"})

	for _, err := range []error{runErr, executeErr} {
		var exitErr *exec.ExitError
		require.ErrorAs(t, err, &exitErr)
		assert.ErrorContains(t, err, "denied")
	}
}

func TestMockClient_Execute(t *testing.T) {
	var capturedAction Action

	mock := &MockClient{
		ExecuteFunc: func(_ context.Context, action Action) error {
			capturedAction = action
			return nil
		},
	}

	err := mock.Execute(context.Background(), CreateSession{Name: "dev", Path: "~/code"})

	assert.NoError(t, err)
	assert.Equal(t, CreateSession{Name: "dev", Path: "~/code"}, capturedAction)
}
