package tui

import (
	"context"
	"fmt"

	"github.com/MSmaili/hetki/internal/backend"
	"github.com/MSmaili/hetki/internal/frecency"
	ui "github.com/MSmaili/hetki/internal/tui"
	"github.com/MSmaili/hetki/internal/tui/list"
)

type LiveAdapter struct {
	DetectBackend func(...string) (backend.Backend, error)
	cached        backend.Backend
	index         itemIndex
	projection    projectionKind
	frecency      *frecency.Store
	frecencyErr   error
	pendingRecord *navigationRecord
}

func NewLiveAdapter(detectBackend func(...string) (backend.Backend, error)) *LiveAdapter {
	store, err := frecency.DefaultStore()
	return newLiveAdapter(detectBackend, store, err)
}

func newLiveAdapter(detectBackend func(...string) (backend.Backend, error), store *frecency.Store, storeErr error) *LiveAdapter {
	return &LiveAdapter{DetectBackend: detectBackend, projection: projectionFlat, frecency: store, frecencyErr: storeErr}
}

func (a *LiveAdapter) Execute(ctx context.Context, request ui.ActionRequest) (ui.ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ui.ActionResult{}, err
	}
	if request.ActionID == ui.ActionToggleProjection {
		return a.toggleProjection(ctx, request.ItemID)
	}
	var item liveItem
	if request.ItemID != "" {
		var err error
		item, err = a.resolveItem(request.ItemID)
		if err != nil {
			return ui.ActionResult{}, err
		}
	}
	switch request.ActionID {
	case ui.ActionLastSession:
		for _, item := range a.index {
			if item.Last {
				return a.openItem(ctx, item)
			}
		}
		return ui.ActionResult{}, fmt.Errorf("no previous session available")
	case ui.ActionRefresh:
		snapshot, err := a.loadSnapshot(ctx)
		return ui.ActionResult{Message: "refreshed", Snapshot: &snapshot}, err
	case ui.ActionCreateSession:
		return a.createSession(ctx, request)
	case ui.ActionContextMenu:
		if item.ID == "" {
			return ui.ActionResult{}, fmt.Errorf("action requires a selected item")
		}
		menu, err := contextualMenu(item)
		if err != nil {
			return ui.ActionResult{}, err
		}
		return ui.ActionResult{Menu: &menu}, nil
	}
	if item.ID == "" {
		return ui.ActionResult{}, fmt.Errorf("action requires a selected item")
	}
	if (request.ActionID == ui.ActionRename || request.ActionID == ui.ActionDelete) &&
		item.Kind != liveWindow && item.Kind != liveDestination {
		return ui.ActionResult{}, fmt.Errorf("item %q does not support a window action", item.ID)
	}
	switch request.ActionID {
	case ui.ActionOpen:
		return a.openItem(ctx, item)
	case ui.ActionCreateWindow:
		return a.createWindow(ctx, request, item)
	case ui.ActionRename:
		return a.renameItem(ctx, request, item)
	case ui.ActionRenameSession:
		session, err := owningSessionItem(item)
		if err != nil {
			return ui.ActionResult{}, err
		}
		return a.renameItem(ctx, request, session)
	case ui.ActionDelete:
		return a.deleteItem(ctx, request, item)
	case ui.ActionDeleteSession:
		session, err := owningSessionItem(item)
		if err != nil {
			return ui.ActionResult{}, err
		}
		return a.deleteItem(ctx, request, session)
	default:
		return ui.ActionResult{}, fmt.Errorf("action %q is not implemented", request.ActionID)
	}
}

func (a *LiveAdapter) resolveItem(id list.ItemID) (liveItem, error) {
	if id == "" {
		return liveItem{}, fmt.Errorf("action requires a selected item")
	}
	item, exists := a.index[id]
	if !exists {
		return liveItem{}, fmt.Errorf("selected item %q is stale", id)
	}
	return item, nil
}

func (a *LiveAdapter) detectBackend() (backend.Backend, error) {
	if a.cached != nil {
		return a.cached, nil
	}
	var b backend.Backend
	var err error
	if a.DetectBackend != nil {
		b, err = a.DetectBackend()
	} else {
		b, err = backend.Detect()
	}
	if err != nil {
		return nil, err
	}
	a.cached = b
	return b, nil
}
