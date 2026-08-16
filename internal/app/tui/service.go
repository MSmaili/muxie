package tui

import (
	"context"
	"errors"
	"fmt"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/MSmaili/hetki/internal/backend"
	"github.com/MSmaili/hetki/internal/tui/core"
	"github.com/MSmaili/hetki/internal/tui/list"
)

type Driver interface {
	Load(context.Context) (list.Snapshot, error)
	Execute(context.Context, core.ActionRequest) (core.ActionResult, error)
	Navigate(context.Context, core.BackendTarget) error
}

type RunUIFunc func(context.Context, list.Snapshot, core.KeyMap, core.DispatchFunc) (core.BackendTarget, error)

type Service struct {
	Driver Driver
	Keys   core.KeyMap
	RunUI  RunUIFunc
}

func NewService(detectBackend func(...string) (backend.Backend, error)) Service {
	return Service{Driver: NewLiveAdapter(detectBackend), Keys: core.DefaultKeyMap(), RunUI: core.RunWithKeyMap}
}

func (s Service) Run(ctx context.Context) error {
	if s.Driver == nil {
		return fmt.Errorf("tui driver is not configured")
	}
	runUI := s.RunUI
	if runUI == nil {
		runUI = core.RunWithKeyMap
	}
	keys := s.Keys
	if keys.IsZero() {
		keys = core.DefaultKeyMap()
	}

	effectsCtx, cancelEffects := context.WithCancel(ctx)
	defer cancelEffects()
	var effects sync.WaitGroup
	var effectsMu sync.Mutex
	effectsClosed := false

	initial, err := s.Driver.Load(effectsCtx)
	if err != nil {
		return err
	}

	dispatch := func(request core.ActionRequest) (core.ActionResult, error) {
		effectsMu.Lock()
		if effectsClosed {
			effectsMu.Unlock()
			return core.ActionResult{}, context.Canceled
		}
		effects.Add(1)
		effectsMu.Unlock()
		defer effects.Done()
		if err := effectsCtx.Err(); err != nil {
			return core.ActionResult{}, err
		}
		return s.Driver.Execute(effectsCtx, request)
	}

	navigation, err := runUI(effectsCtx, initial, keys, dispatch)
	effectsMu.Lock()
	effectsClosed = true
	cancelEffects()
	effectsMu.Unlock()
	effects.Wait()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if errors.Is(err, tea.ErrProgramKilled) || errors.Is(err, tea.ErrInterrupted) {
			return nil
		}
		return err
	}
	if navigation != "" {
		return s.Driver.Navigate(ctx, navigation)
	}
	return nil
}
