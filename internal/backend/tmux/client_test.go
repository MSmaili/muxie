package tmux

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockClient for testing
type MockClient struct {
	RunFunc          func(args ...string) (string, error)
	ExecuteFunc      func(action Action) error
	ExecuteBatchFunc func(actions []Action) error
}

func (m *MockClient) Run(args ...string) (string, error) {
	if m.RunFunc != nil {
		return m.RunFunc(args...)
	}
	return "", nil
}

func (m *MockClient) Execute(action Action) error {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(action)
	}
	return nil
}

func (m *MockClient) ExecuteBatch(actions []Action) error {
	if m.ExecuteBatchFunc != nil {
		return m.ExecuteBatchFunc(actions)
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
			output: "0\n0\n$1|dev|@1|editor|0|layout-a|0|1|%1|0|1|~/code|vim",
			want: LoadStateResult{
				Sessions: []Session{{ID: "$1", Name: "dev", Windows: []Window{{ID: "@1", Name: "editor", Index: 0, Path: "~/code", Layout: "layout-a", Panes: []Pane{{ID: "%1", Index: 0, Path: "~/code", Command: "vim"}}}}}},
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
				RunFunc: func(args ...string) (string, error) {
					return tt.output, tt.runErr
				},
			}

			got, err := RunQuery(mock, LoadStateQuery{})

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestMockClient_Execute(t *testing.T) {
	var capturedAction Action

	mock := &MockClient{
		ExecuteFunc: func(action Action) error {
			capturedAction = action
			return nil
		},
	}

	err := mock.Execute(CreateSession{Name: "dev", Path: "~/code"})

	assert.NoError(t, err)
	assert.Equal(t, CreateSession{Name: "dev", Path: "~/code"}, capturedAction)
}

func TestBuildBatchArgs(t *testing.T) {
	assert.Equal(t, []string{
		"new-session", "-d", "-s", "dev", "-n", "editor", ";",
		"send-keys", "-t", "dev:0", "npm test", "Enter", ";",
		"select-layout", "-t", "dev:0", "tiled",
	}, buildBatchArgs([]Action{
		CreateSession{Name: "dev", WindowName: "editor"},
		SendKeys{Target: "dev:0", Keys: "npm test"},
		SelectLayout{Target: "dev:0", Layout: "tiled"},
	}))
}
