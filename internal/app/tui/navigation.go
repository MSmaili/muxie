package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/MSmaili/hetki/internal/backend"
	ui "github.com/MSmaili/hetki/internal/tui"
)

type navigationRecord struct {
	target  string
	path    string
	session string
}

func (a *LiveAdapter) openItem(ctx context.Context, item liveItem) (ui.ActionResult, error) {
	a.pendingRecord = nil
	if strings.TrimSpace(item.Target) == "" {
		return ui.ActionResult{}, fmt.Errorf("item %q has no navigation target", item.ID)
	}
	b, err := a.detectBackend()
	if err != nil {
		return ui.ActionResult{}, fmt.Errorf("failed to detect backend: %w", err)
	}
	state, err := b.QueryState(ctx)
	if err != nil {
		return ui.ActionResult{}, fmt.Errorf("validate selected item %q: %w", item.ID, err)
	}
	if !liveItemExists(state, item) {
		return ui.ActionResult{}, fmt.Errorf("selected item %q is stale", item.ID)
	}
	if record, ok := navigationRecordForItem(state, item); ok {
		a.pendingRecord = &record
	}
	return ui.ActionResult{Message: "switching to " + item.Target, Navigation: ui.BackendTarget(item.Target)}, nil
}

func liveItemExists(state backend.StateResult, item liveItem) bool {
	session, found := findSession(state.Sessions, item.SessionTarget)
	if !found {
		return false
	}
	if item.Kind == liveSession {
		return item.Target == session.ID
	}

	if item.Kind == liveWindow {
		_, found := findWindow(session, item.Target)
		return found
	}
	if item.Kind != liveDestination {
		return false
	}

	window, found := findWindow(session, item.MutationTarget)
	if !found {
		return false
	}
	for _, pane := range window.Panes {
		if pane.ID == item.Target && pane.Path == item.RawPath {
			return true
		}
	}
	return false
}

func navigationRecordForItem(state backend.StateResult, item liveItem) (navigationRecord, bool) {
	session, found := findSession(state.Sessions, item.SessionTarget)
	if !found {
		return navigationRecord{}, false
	}

	if item.Kind == liveSession {
		pane, found := activeWindowPane(session.Windows)
		if !found {
			return navigationRecord{}, false
		}
		return navigationRecord{target: item.Target, path: pane.Path, session: session.Name}, true
	}

	window, found := findWindow(session, item.MutationTarget)
	if !found {
		return navigationRecord{}, false
	}
	if item.Kind == liveDestination {
		record := navigationRecord{target: item.Target, path: item.RawPath, session: session.Name}
		return record, item.RawPath != ""
	}
	pane, found := activePane(window.Panes)
	if !found || pane.Path == "" {
		return navigationRecord{}, false
	}
	return navigationRecord{target: item.Target, path: pane.Path, session: session.Name}, true
}

func findSession(sessions []backend.Session, target string) (backend.Session, bool) {
	for _, session := range sessions {
		if session.ID == target {
			return session, true
		}
	}
	return backend.Session{}, false
}

func findWindow(session backend.Session, target string) (backend.Window, bool) {
	for _, window := range session.Windows {
		if session.ID+":"+window.ID == target {
			return window, true
		}
	}
	return backend.Window{}, false
}

func activeWindowPane(windows []backend.Window) (backend.Pane, bool) {
	for _, window := range windows {
		if !window.Active {
			continue
		}
		if pane, found := activePane(window.Panes); found && pane.Path != "" {
			return pane, true
		}
	}
	return backend.Pane{}, false
}

func activePane(panes []backend.Pane) (backend.Pane, bool) {
	for _, pane := range panes {
		if pane.Active {
			return pane, true
		}
	}
	return backend.Pane{}, false
}

func (a *LiveAdapter) Navigate(ctx context.Context, navigation ui.BackendTarget) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	target := strings.TrimSpace(string(navigation))
	if target == "" {
		return fmt.Errorf("empty navigation target")
	}
	record := a.pendingRecord
	a.pendingRecord = nil
	b, err := a.detectBackend()
	if err != nil {
		return fmt.Errorf("failed to detect backend: %w", err)
	}
	if err := b.Switch(ctx, target); err != nil {
		return fmt.Errorf("switch to %q: %w", target, err)
	}
	if record != nil && record.target == target && a.frecency != nil {
		if err := a.frecency.Record(ctx, record.path, record.session); err != nil {
			return fmt.Errorf("record navigation to %q: %w", target, err)
		}
	}
	return nil
}
