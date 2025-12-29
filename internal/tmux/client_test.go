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
	AttachFunc       func(session string) error
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

func (m *MockClient) Attach(session string) error {
	if m.AttachFunc != nil {
		return m.AttachFunc(session)
	}
	return nil
}

func TestRunQuery(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		runErr  error
		want    []Session
		wantErr bool
	}{
		{
			name:   "success",
			output: "dev|editor|~/code|vim|",
			want:   []Session{{Name: "dev", WorkspacePath: "", Windows: []Window{{Name: "editor", Path: "~/code", Panes: []Pane{{Path: "~/code", Command: "vim"}}}}}},
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
