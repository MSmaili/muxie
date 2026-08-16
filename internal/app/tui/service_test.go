package tui

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/MSmaili/hetki/internal/tui/core"
	"github.com/MSmaili/hetki/internal/tui/list"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func awaitChannel[T any](t *testing.T, ch <-chan T) T {
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

type switchDriver struct {
	loads       int
	navigations []core.BackendTarget
	uiReturned  bool
}

func (d *switchDriver) Load(context.Context) (list.Snapshot, error) {
	d.loads++
	return list.Snapshot{}, nil
}

func (d *switchDriver) Execute(context.Context, core.ActionRequest) (core.ActionResult, error) {
	return core.ActionResult{Navigation: "dev"}, nil
}

func (d *switchDriver) Navigate(_ context.Context, outcome core.BackendTarget) error {
	if !d.uiReturned {
		return assert.AnError
	}
	d.navigations = append(d.navigations, outcome)
	return nil
}

type blockingDriver struct {
	started chan struct{}
	stopped chan struct{}
}

func (d *blockingDriver) Load(context.Context) (list.Snapshot, error) {
	return list.Snapshot{}, nil
}

func (d *blockingDriver) Execute(ctx context.Context, _ core.ActionRequest) (core.ActionResult, error) {
	close(d.started)
	<-ctx.Done()
	close(d.stopped)
	return core.ActionResult{}, ctx.Err()
}

func (d *blockingDriver) Navigate(context.Context, core.BackendTarget) error { return nil }

func TestUIExitCancelsAndJoinsBusyEffect(t *testing.T) {
	driver := &blockingDriver{started: make(chan struct{}), stopped: make(chan struct{})}
	dispatchDone := make(chan error, 1)
	service := Service{
		Driver: driver,
		RunUI: func(_ context.Context, _ list.Snapshot, _ core.KeyMap, dispatch core.DispatchFunc) (core.BackendTarget, error) {
			go func() {
				_, err := dispatch(core.ActionRequest{ActionID: core.ActionCreateSession})
				dispatchDone <- err
			}()
			awaitChannel(t, driver.started)
			return "", nil
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, service.Run(ctx))
	select {
	case <-driver.stopped:
	default:
		t.Fatal("busy effect outlived TUI run")
	}
	require.ErrorIs(t, awaitChannel(t, dispatchDone), context.Canceled)
}

type navigationDriver struct {
	started chan struct{}
}

func (d *navigationDriver) Load(context.Context) (list.Snapshot, error) {
	return list.Snapshot{}, nil
}
func (d *navigationDriver) Execute(context.Context, core.ActionRequest) (core.ActionResult, error) {
	return core.ActionResult{}, nil
}
func (d *navigationDriver) Navigate(ctx context.Context, _ core.BackendTarget) error {
	close(d.started)
	<-ctx.Done()
	return ctx.Err()
}

func TestRunPreservesParentCancellation(t *testing.T) {
	started := make(chan struct{})
	service := Service{
		Driver: &switchDriver{},
		RunUI: func(ctx context.Context, _ list.Snapshot, _ core.KeyMap, _ core.DispatchFunc) (core.BackendTarget, error) {
			close(started)
			<-ctx.Done()
			return "", tea.ErrProgramKilled
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()

	awaitChannel(t, started)
	cancel()

	require.ErrorIs(t, awaitChannel(t, done), context.Canceled)
}

func TestPostExitNavigationUsesParentCancellation(t *testing.T) {
	driver := &navigationDriver{started: make(chan struct{})}
	service := Service{
		Driver: driver,
		RunUI: func(context.Context, list.Snapshot, core.KeyMap, core.DispatchFunc) (core.BackendTarget, error) {
			return "dev", nil
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()

	awaitChannel(t, driver.started)
	cancel()

	require.ErrorIs(t, awaitChannel(t, done), context.Canceled)
}

func TestSuccessfulSwitchNavigatesAfterUIWithoutRefresh(t *testing.T) {
	driver := &switchDriver{}
	service := Service{
		Driver: driver,
		RunUI: func(_ context.Context, _ list.Snapshot, _ core.KeyMap, dispatch core.DispatchFunc) (core.BackendTarget, error) {
			defer func() { driver.uiReturned = true }()
			result, err := dispatch(core.ActionRequest{ActionID: core.ActionOpen, ItemID: "dev"})
			return result.Navigation, err
		},
	}

	require.NoError(t, service.Run(context.Background()))
	assert.Equal(t, 1, driver.loads)
	assert.Equal(t, []core.BackendTarget{"dev"}, driver.navigations)
}
