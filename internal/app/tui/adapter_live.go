package tui

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/MSmaili/hetki/internal/backend"
	"github.com/MSmaili/hetki/internal/tui/core"
	"github.com/MSmaili/hetki/internal/tui/list"
)

type projectionKind uint8

const (
	projectionTree projectionKind = iota
	projectionFlat
)

type LiveAdapter struct {
	DetectBackend func(...string) (backend.Backend, error)
	cached        backend.Backend
	index         itemIndex
	projection    projectionKind
}

func NewLiveAdapter(detectBackend func(...string) (backend.Backend, error)) *LiveAdapter {
	return &LiveAdapter{DetectBackend: detectBackend}
}

func (a *LiveAdapter) Load(ctx context.Context) (list.Snapshot, error) {
	return a.loadSnapshot(ctx)
}

func (a *LiveAdapter) Execute(ctx context.Context, request core.ActionRequest) (core.ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return core.ActionResult{}, err
	}
	if request.ActionID == core.ActionRefresh {
		snapshot, err := a.loadSnapshot(ctx)
		return core.ActionResult{Message: "refreshed", Snapshot: &snapshot}, err
	}
	if request.ActionID == core.ActionCreateSession {
		return a.createSession(ctx, request)
	}
	if request.ActionID == core.ActionToggleProjection {
		return a.toggleProjection(ctx, request.ItemID)
	}

	item, err := a.resolveItem(request.ItemID)
	if err != nil {
		return core.ActionResult{}, err
	}
	switch request.ActionID {
	case core.ActionOpen:
		if strings.TrimSpace(item.Target) == "" {
			return core.ActionResult{}, fmt.Errorf("item %q has no navigation target", item.ID)
		}
		b, err := a.detectBackend()
		if err != nil {
			return core.ActionResult{}, fmt.Errorf("failed to detect backend: %w", err)
		}
		state, err := b.QueryState(ctx)
		if err != nil {
			return core.ActionResult{}, fmt.Errorf("validate selected item %q: %w", item.ID, err)
		}
		if !liveItemExists(state, item) {
			return core.ActionResult{}, fmt.Errorf("selected item %q is stale", item.ID)
		}
		return core.ActionResult{Message: "switching to " + item.Target, Navigation: core.BackendTarget(item.Target)}, nil
	case core.ActionCreateWindow:
		return a.createWindow(ctx, request, item)
	case core.ActionRename:
		return a.renameItem(ctx, request, item)
	case core.ActionDelete:
		return a.deleteItem(ctx, request, item)
	default:
		return core.ActionResult{}, fmt.Errorf("action %q is not implemented", request.ActionID)
	}
}

func liveItemExists(state backend.StateResult, item liveItem) bool {
	for _, session := range state.Sessions {
		if session.ID != item.SessionTarget {
			continue
		}
		if item.Kind == liveSession {
			return item.Target == session.ID
		}
		for _, window := range session.Windows {
			windowTarget := session.ID + ":" + window.ID
			if item.Kind == liveWindow && item.Target == windowTarget {
				return true
			}
			if item.Kind != liveDestination || item.MutationTarget != windowTarget {
				continue
			}
			for _, pane := range window.Panes {
				if pane.ID == item.Target && pane.Path == item.RawPath {
					return true
				}
			}
		}
		return false
	}
	return false
}

func (a *LiveAdapter) Navigate(ctx context.Context, navigation core.BackendTarget) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	target := strings.TrimSpace(string(navigation))
	if target == "" {
		return fmt.Errorf("empty navigation target")
	}
	b, err := a.detectBackend()
	if err != nil {
		return fmt.Errorf("failed to detect backend: %w", err)
	}
	if err := b.Switch(ctx, target); err != nil {
		return fmt.Errorf("switch to %q: %w", target, err)
	}
	return nil
}

func (a *LiveAdapter) createSession(ctx context.Context, request core.ActionRequest) (core.ActionResult, error) {
	if request.Value == nil {
		return core.ActionResult{Input: &core.InputPrompt{
			Title: "CREATE SESSION", Prompt: "Session name", SubmitStatus: "creating session...",
		}}, nil
	}
	name := strings.TrimSpace(*request.Value)
	if name == "" {
		return core.ActionResult{}, fmt.Errorf("session name is required")
	}
	b, err := a.detectBackend()
	if err != nil {
		return core.ActionResult{}, fmt.Errorf("failed to detect backend: %w", err)
	}
	actions := []backend.Action{backend.CreateSessionAction{Name: name}}
	workspacePath, err := activeWorkspacePath(ctx, b)
	if err != nil {
		return core.ActionResult{}, err
	}
	if workspacePath != "" {
		actions = append(actions, backend.SetSessionOptionAction{Session: name, Key: backend.WorkspacePathOption, Value: workspacePath})
	}
	if err := b.Apply(ctx, actions); err != nil {
		return core.ActionResult{}, fmt.Errorf("create session %q: %w", name, err)
	}
	message := "created session " + name
	if workspacePath != "" {
		message += " (workspace linked)"
	}
	return a.refreshAfter(ctx, message, func(snapshot list.Snapshot, index itemIndex) list.ItemID {
		return firstMatchingItem(snapshot, index, func(item liveItem) bool {
			return item.SessionName == name
		})
	})
}

func (a *LiveAdapter) createWindow(ctx context.Context, request core.ActionRequest, selected liveItem) (core.ActionResult, error) {
	if request.Value == nil {
		return core.ActionResult{Input: &core.InputPrompt{
			Title: "CREATE WINDOW", Prompt: "Window name", SubmitStatus: "creating window...",
		}}, nil
	}
	name := strings.TrimSpace(*request.Value)
	if name == "" {
		return core.ActionResult{}, fmt.Errorf("window name is required")
	}
	b, err := a.detectBackend()
	if err != nil {
		return core.ActionResult{}, fmt.Errorf("failed to detect backend: %w", err)
	}
	existing := make(map[list.ItemID]struct{}, len(a.index))
	for _, item := range a.index {
		if item.WindowID != "" {
			existing[item.WindowID] = struct{}{}
		}
	}
	if err := b.Apply(ctx, []backend.Action{backend.CreateWindowAction{Session: selected.SessionTarget, Name: name}}); err != nil {
		return core.ActionResult{}, fmt.Errorf("create window %q in %q: %w", name, selected.SessionTarget, err)
	}
	return a.refreshAfter(ctx, "created window "+selected.SessionTarget+":"+name, func(snapshot list.Snapshot, index itemIndex) list.ItemID {
		return firstMatchingItem(snapshot, index, func(item liveItem) bool {
			_, existed := existing[item.WindowID]
			return item.SessionTarget == selected.SessionTarget && item.WindowName == name && !existed
		})
	})
}

func (a *LiveAdapter) renameItem(ctx context.Context, request core.ActionRequest, item liveItem) (core.ActionResult, error) {
	if request.Value == nil {
		title, prompt, status := "RENAME WINDOW", "Window name", "renaming window..."
		if item.Kind == liveSession {
			title, prompt, status = "RENAME SESSION", "Session name", "renaming session..."
		}
		return core.ActionResult{Input: &core.InputPrompt{
			Title: title, Prompt: prompt, InitialValue: item.Name, SubmitStatus: status,
		}}, nil
	}
	name := strings.TrimSpace(*request.Value)
	if name == "" {
		return core.ActionResult{}, fmt.Errorf("new name is required")
	}
	b, err := a.detectBackend()
	if err != nil {
		return core.ActionResult{}, fmt.Errorf("failed to detect backend: %w", err)
	}
	var action backend.Action
	var message string
	if item.Kind == liveSession {
		action = backend.RenameSessionAction{Current: item.Target, New: name}
		message = "renamed session " + item.Target + " -> " + name
	} else {
		session, window, err := sessionWindowFromTarget(core.BackendTarget(item.MutationTarget))
		if err != nil {
			return core.ActionResult{}, err
		}
		action = backend.RenameWindowAction{Session: session, Window: window, WindowID: window, New: name}
		message = "renamed window " + session + ":" + window + " -> " + name
	}
	if err := b.Apply(ctx, []backend.Action{action}); err != nil {
		return core.ActionResult{}, fmt.Errorf("rename item %q: %w", item.ID, err)
	}
	return a.refreshAfter(ctx, message, func(list.Snapshot, itemIndex) list.ItemID { return item.ID })
}

func (a *LiveAdapter) deleteItem(ctx context.Context, request core.ActionRequest, item liveItem) (core.ActionResult, error) {
	if !request.Confirmed {
		title, noun, status := "DELETE WINDOW", "window", "deleting window..."
		if item.Kind == liveSession {
			title, noun, status = "DELETE SESSION", "session", "deleting session..."
		}
		return core.ActionResult{Confirmation: &core.Confirmation{
			Title: title, Body: fmt.Sprintf("Delete %s %q?", noun, item.Label), SubmitStatus: status,
		}}, nil
	}
	b, err := a.detectBackend()
	if err != nil {
		return core.ActionResult{}, fmt.Errorf("failed to detect backend: %w", err)
	}
	var action backend.Action
	var message string
	preferred := item.ParentID
	if item.Kind == liveSession {
		action = backend.KillSessionAction{Name: item.Target}
		message = "deleted session " + item.Target
		preferred = ""
	} else {
		session, window, err := sessionWindowFromTarget(core.BackendTarget(item.MutationTarget))
		if err != nil {
			return core.ActionResult{}, err
		}
		action = backend.KillWindowAction{Session: session, Window: window, WindowID: window}
		message = "deleted window " + session + ":" + window
	}
	if err := b.Apply(ctx, []backend.Action{action}); err != nil {
		return core.ActionResult{}, fmt.Errorf("delete item %q: %w", item.ID, err)
	}
	return a.refreshAfter(ctx, message, func(list.Snapshot, itemIndex) list.ItemID { return preferred })
}

func (a *LiveAdapter) refreshAfter(ctx context.Context, message string, selectItem func(list.Snapshot, itemIndex) list.ItemID) (core.ActionResult, error) {
	snapshot, err := a.loadSnapshot(ctx)
	if err != nil {
		return core.ActionResult{}, err
	}
	return core.ActionResult{Message: message, Snapshot: &snapshot, SelectItemID: selectItem(snapshot, a.index)}, nil
}

func (a *LiveAdapter) loadSnapshot(ctx context.Context) (list.Snapshot, error) {
	result, err := a.queryState(ctx)
	if err != nil {
		return list.Snapshot{}, err
	}
	snapshot, index, err := projectState(result, a.projection)
	if err != nil {
		return list.Snapshot{}, err
	}
	a.index = index
	return snapshot, nil
}

func (a *LiveAdapter) queryState(ctx context.Context) (backend.StateResult, error) {
	if err := ctx.Err(); err != nil {
		return backend.StateResult{}, err
	}
	b, err := a.detectBackend()
	if err != nil {
		return backend.StateResult{}, fmt.Errorf("failed to detect backend: %w", err)
	}
	result, err := b.QueryState(ctx)
	if err != nil {
		return backend.StateResult{}, fmt.Errorf("failed to query sessions: %w", err)
	}
	return result, nil
}

func projectState(result backend.StateResult, projection projectionKind) (list.Snapshot, itemIndex, error) {
	homeDir, _ := os.UserHomeDir()
	var snapshot list.Snapshot
	var index itemIndex
	var err error
	if projection == projectionFlat {
		snapshot, index, err = projectFlat(result, homeDir)
	} else {
		snapshot, index, err = projectTree(result, homeDir)
	}
	if err != nil {
		return list.Snapshot{}, nil, fmt.Errorf("project live sessions: %w", err)
	}
	return snapshot, index, nil
}

func (a *LiveAdapter) toggleProjection(ctx context.Context, selectedID list.ItemID) (core.ActionResult, error) {
	var selected liveItem
	if selectedID != "" {
		var err error
		selected, err = a.resolveItem(selectedID)
		if err != nil {
			return core.ActionResult{}, err
		}
	}
	next := projectionFlat
	message := "showing flat destinations"
	if a.projection == projectionFlat {
		next = projectionTree
		message = "showing session tree"
	}
	state, err := a.queryState(ctx)
	if err != nil {
		return core.ActionResult{}, err
	}
	snapshot, index, err := projectState(state, next)
	if err != nil {
		return core.ActionResult{}, err
	}
	preferred := preferredProjectionItem(selected, next, snapshot, index)
	a.projection = next
	a.index = index
	return core.ActionResult{Message: message, Snapshot: &snapshot, SelectItemID: preferred}, nil
}

func preferredProjectionItem(selected liveItem, projection projectionKind, snapshot list.Snapshot, index itemIndex) list.ItemID {
	if projection == projectionTree {
		if selected.WindowID != "" {
			if _, exists := index[selected.WindowID]; exists {
				return selected.WindowID
			}
		}
		if selected.SessionID != "" {
			if _, exists := index[selected.SessionID]; exists {
				return selected.SessionID
			}
		}
		if snapshot.ActiveItemID != "" {
			return snapshot.ActiveItemID
		}
		return firstMatchingItem(snapshot, index, func(liveItem) bool { return true })
	}

	if selected.WindowID != "" {
		if id := firstMatchingItem(snapshot, index, func(item liveItem) bool {
			return item.WindowID == selected.WindowID && item.PaneActive
		}); id != "" {
			return id
		}
	} else if selected.SessionID != "" {
		if id := firstMatchingItem(snapshot, index, func(item liveItem) bool {
			return item.SessionID == selected.SessionID && item.WindowActive && item.PaneActive
		}); id != "" {
			return id
		}
	}
	if snapshot.ActiveItemID != "" {
		return snapshot.ActiveItemID
	}
	return firstMatchingItem(snapshot, index, func(liveItem) bool { return true })
}

func firstMatchingItem(snapshot list.Snapshot, index itemIndex, matches func(liveItem) bool) list.ItemID {
	var visit func([]list.Item) list.ItemID
	visit = func(items []list.Item) list.ItemID {
		for _, item := range items {
			if live, exists := index[item.ID]; exists && matches(live) {
				return item.ID
			}
			if id := visit(item.Children); id != "" {
				return id
			}
		}
		return ""
	}
	return visit(snapshot.Items)
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

func workspacePathForSession(sessions []backend.Session, sessionName string) string {
	for _, session := range sessions {
		if session.Name == sessionName {
			return session.WorkspacePath
		}
	}
	return ""
}

func activeWorkspacePath(ctx context.Context, b backend.Backend) (string, error) {
	state, err := b.QueryState(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to query sessions for workspace inheritance: %w", err)
	}
	return strings.TrimSpace(workspacePathForSession(state.Sessions, state.Active.Session)), nil
}

func displayPath(path, home string) string {
	path = strings.TrimSpace(path)
	if path == "" || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+"/") {
		return "~" + path[len(home):]
	}
	return path
}

func sessionWindowFromTarget(target core.BackendTarget) (string, string, error) {
	value := strings.TrimSpace(string(target))
	session, rest, hasWindow := strings.Cut(value, ":")
	if !hasWindow || strings.TrimSpace(session) == "" || strings.TrimSpace(rest) == "" {
		return "", "", fmt.Errorf("window target must be in session:window format")
	}
	window, _, _ := strings.Cut(rest, ".")
	window = strings.TrimSpace(window)
	idText := strings.TrimPrefix(window, "@")
	id, err := strconv.Atoi(idText)
	if !strings.HasPrefix(window, "@") || err != nil || id < 0 || strconv.Itoa(id) != idText {
		return "", "", fmt.Errorf("window target must use a stable @N window ID")
	}
	return strings.TrimSpace(session), window, nil
}
