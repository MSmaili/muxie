package tui

import (
	"context"
	"testing"

	"github.com/MSmaili/hetki/internal/tui/contracts"
	"github.com/MSmaili/hetki/internal/tui/core"
	"github.com/stretchr/testify/require"
)

type switchDriver struct{ refreshes int }

func (d *switchDriver) Load(context.Context) (contracts.Snapshot, error) {
	return contracts.Snapshot{}, nil
}

func (d *switchDriver) Refresh(context.Context) (contracts.Snapshot, error) {
	d.refreshes++
	return contracts.Snapshot{}, nil
}

func (d *switchDriver) Execute(context.Context, contracts.Intent) (contracts.ActionResult, error) {
	return contracts.ActionResult{NeedsRefresh: true}, nil
}

func TestSuccessfulSwitchDoesNotRefresh(t *testing.T) {
	driver := &switchDriver{}
	service := Service{
		Provider: driver,
		Executor: driver,
		RunUI: func(ctx context.Context, _ contracts.Snapshot, dispatch core.DispatchFunc) error {
			_, err := dispatch(ctx, contracts.Intent{Type: contracts.IntentSwitch, Target: "dev"})
			return err
		},
	}

	require.NoError(t, service.Run(context.Background()))
	if driver.refreshes != 0 {
		t.Fatalf("switch triggered %d refreshes", driver.refreshes)
	}
}
