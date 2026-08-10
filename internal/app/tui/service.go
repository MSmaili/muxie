package tui

import (
	"context"
	"errors"
	"fmt"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/MSmaili/hetki/internal/backend"
	"github.com/MSmaili/hetki/internal/tui/contracts"
	"github.com/MSmaili/hetki/internal/tui/core"
)

type Driver interface {
	Load(context.Context) (contracts.Snapshot, error)
	Execute(context.Context, contracts.Intent) (contracts.ActionResult, error)
	Navigate(context.Context, contracts.BackendTarget) error
}

type Service struct {
	Driver Driver
	RunUI  func(context.Context, contracts.Snapshot, core.DispatchFunc) (contracts.BackendTarget, error)
}

func NewService(detectBackend func(...string) (backend.Backend, error)) Service {
	return Service{Driver: NewLiveAdapter(detectBackend), RunUI: core.Run}
}

func (s Service) Run(ctx context.Context) error {
	if s.Driver == nil {
		return fmt.Errorf("tui driver is not configured")
	}
	runUI := s.RunUI
	if runUI == nil {
		runUI = core.Run
	}

	effectsCtx, cancelEffects := context.WithCancel(ctx)
	defer cancelEffects()
	var effects sync.WaitGroup
	var effectsMu sync.Mutex
	var effectsClosed bool

	initial, err := s.Driver.Load(effectsCtx)
	if err != nil {
		return err
	}

	dispatch := func(intent contracts.Intent) (contracts.ActionResult, error) {
		effectsMu.Lock()
		if effectsClosed {
			effectsMu.Unlock()
			return contracts.ActionResult{}, context.Canceled
		}
		effects.Add(1)
		effectsMu.Unlock()
		defer effects.Done()
		if err := effectsCtx.Err(); err != nil {
			return contracts.ActionResult{}, err
		}
		if intent.Type == contracts.IntentRefresh {
			snapshot, err := s.Driver.Load(effectsCtx)
			if err != nil {
				return contracts.ActionResult{}, err
			}
			return contracts.ActionResult{Message: "refreshed", Snapshot: &snapshot}, nil
		}

		result, err := s.Driver.Execute(effectsCtx, intent)
		if err != nil {
			return contracts.ActionResult{}, err
		}
		if result.Navigation == "" && result.NeedsRefresh && result.Snapshot == nil {
			snapshot, err := s.Driver.Load(effectsCtx)
			if err != nil {
				return contracts.ActionResult{}, err
			}
			result.Snapshot = &snapshot
		}
		return result, nil
	}

	navigation, err := runUI(effectsCtx, initial, dispatch)
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
