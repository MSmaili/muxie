package list

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/MSmaili/hetki/internal/backend"
	"github.com/MSmaili/hetki/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubBackend struct {
	queryResult backend.StateResult
	queryErr    error
}

func (s *stubBackend) Name() string { return "stub" }

func (s *stubBackend) QueryState(context.Context) (backend.StateResult, error) {
	if s.queryErr != nil {
		return backend.StateResult{}, s.queryErr
	}
	return s.queryResult, nil
}

func (s *stubBackend) Apply(context.Context, []backend.Action) error { return nil }
func (s *stubBackend) DryRun([]backend.Action) ([]string, error)     { return nil, nil }
func (s *stubBackend) Attach(context.Context, string) error          { return nil }
func (s *stubBackend) Switch(context.Context, string) error          { return nil }

func TestServiceRunWorkspacesReturnsSortedNames(t *testing.T) {
	service := Service{
		GetConfigDir: func() (string, error) { return "/config", nil },
		ScanWorkspaces: func(dir string) (map[string]string, error) {
			assert.Equal(t, "/config/workspaces", dir)
			return map[string]string{"zeta": "/zeta.yaml", "alpha": "/alpha.yaml"}, nil
		},
	}

	result, err := service.Run(context.Background(), Options{})
	require.NoError(t, err)
	assert.True(t, result.NamesOnly)
	assert.Equal(t, []string{"alpha", "zeta"}, result.Names)
	assert.Empty(t, result.Items)
}

func TestServiceRunWorkspacesReturnsPartialResultsAndLoadErrors(t *testing.T) {
	service := Service{
		GetConfigDir: func() (string, error) { return "/config", nil },
		ScanWorkspaces: func(string) (map[string]string, error) {
			return map[string]string{
				"valid":    "/valid.yaml",
				"broken-a": "/broken-a.yaml",
				"broken-b": "/broken-b.yaml",
			}, nil
		},
		LoadWorkspace: func(path string) (*manifest.Workspace, error) {
			if path != "/valid.yaml" {
				return nil, errors.New("invalid manifest")
			}
			return &manifest.Workspace{Sessions: []manifest.Session{{Name: "dev"}}}, nil
		},
	}

	result, err := service.Run(context.Background(), Options{IncludeSessions: true})
	assert.Equal(t, []Item{{Name: "valid:dev"}}, result.Items)
	require.EqualError(t, err, "workspace \"broken-a\" (/broken-a.yaml): invalid manifest\nworkspace \"broken-b\" (/broken-b.yaml): invalid manifest")
}

func TestServiceRunWorkspacesReportsCancellationAfterScan(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	service := Service{
		GetConfigDir: func() (string, error) { return "/config", nil },
		ScanWorkspaces: func(string) (map[string]string, error) {
			cancel()
			return map[string]string{"dev": "/dev.yaml"}, nil
		},
	}

	_, err := service.Run(ctx, Options{})

	require.ErrorIs(t, err, context.Canceled)
}

func TestServiceRunWorkspacesBoundsConcurrentLoads(t *testing.T) {
	const total = workspaceLoadLimit + 2
	paths := make(map[string]string, total)
	for i := range total {
		name := fmt.Sprintf("workspace-%02d", i)
		paths[name] = "/" + name + ".yaml"
	}

	started := make(chan struct{}, total)
	release := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()

	service := Service{
		GetConfigDir:   func() (string, error) { return "/config", nil },
		ScanWorkspaces: func(string) (map[string]string, error) { return paths, nil },
		LoadWorkspace: func(string) (*manifest.Workspace, error) {
			started <- struct{}{}
			<-release
			return &manifest.Workspace{Sessions: []manifest.Session{{Name: "dev"}}}, nil
		},
	}

	done := make(chan error, 1)
	go func() {
		_, err := service.Run(context.Background(), Options{IncludeSessions: true})
		done <- err
	}()

	for range workspaceLoadLimit {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("workspace loads did not reach concurrency limit")
		}
	}
	select {
	case <-started:
		t.Fatal("workspace loads exceeded concurrency limit")
	default:
	}

	close(release)
	released = true
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("workspace listing did not finish")
	}
}

func TestServiceRunWorkspacesReportsCancellationAfterActiveLoad(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	service := Service{
		GetConfigDir:   func() (string, error) { return "/config", nil },
		ScanWorkspaces: func(string) (map[string]string, error) { return map[string]string{"dev": "/dev.yaml"}, nil },
		LoadWorkspace: func(string) (*manifest.Workspace, error) {
			close(started)
			<-release
			return &manifest.Workspace{}, nil
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := service.Run(ctx, Options{IncludeSessions: true})
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("workspace load did not start")
	}

	cancel()
	close(release)

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("workspace listing did not return after cancellation")
	}
}

func TestServiceRunSessionsReturnsWindowAndPaneState(t *testing.T) {
	stub := &stubBackend{queryResult: backend.StateResult{
		Sessions: []backend.Session{{
			Name: "dev",
			Windows: []backend.Window{{
				Name:  "editor",
				Panes: []backend.Pane{{Index: 0}, {Index: 1}},
			}},
		}},
		Active: backend.ActiveContext{Session: "dev", Window: "editor", Pane: 1},
	}}

	service := NewService(func(...string) (backend.Backend, error) { return stub, nil })
	result, err := service.Run(context.Background(), Options{Mode: ModeSessions, IncludeWindows: true, IncludePanes: true})
	require.NoError(t, err)
	assert.False(t, result.NamesOnly)
	assert.Equal(t, []Item{{
		Name: "dev",
		Windows: []Window{{
			Name:       "editor",
			Panes:      []int{0, 1},
			ActivePane: 1,
		}},
	}}, result.Items)
}
