package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/MSmaili/hetki/internal/backend"
	"github.com/MSmaili/hetki/internal/frecency"
	"github.com/MSmaili/hetki/internal/tui/core"
	"github.com/MSmaili/hetki/internal/tui/list"
)

type projectionKind uint8

const (
	projectionTree projectionKind = iota
	projectionFlat
)

type navigationRecord struct {
	target  string
	path    string
	session string
}

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

func (a *LiveAdapter) Load(ctx context.Context) (list.Snapshot, error) {
	return a.loadSnapshot(ctx)
}

func (a *LiveAdapter) Execute(ctx context.Context, request core.ActionRequest) (core.ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return core.ActionResult{}, err
	}
	if request.ActionID == core.ActionToggleProjection {
		return a.toggleProjection(ctx, request.ItemID)
	}
	var item liveItem
	if request.ItemID != "" {
		var err error
		item, err = a.resolveItem(request.ItemID)
		if err != nil {
			return core.ActionResult{}, err
		}
	}
	switch request.ActionID {
	case core.ActionRefresh:
		snapshot, err := a.loadSnapshot(ctx)
		return core.ActionResult{Message: "refreshed", Snapshot: &snapshot}, err
	case core.ActionCreateSession:
		return a.createSession(ctx, request)
	case core.ActionContextMenu:
		if item.ID == "" {
			return core.ActionResult{}, fmt.Errorf("action requires a selected item")
		}
		menu, err := contextualMenu(item)
		if err != nil {
			return core.ActionResult{}, err
		}
		return core.ActionResult{Menu: &menu}, nil
	}
	if item.ID == "" {
		return core.ActionResult{}, fmt.Errorf("action requires a selected item")
	}
	switch request.ActionID {
	case core.ActionOpen:
		a.pendingRecord = nil
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
		if record, ok := navigationRecordForItem(state, item); ok {
			a.pendingRecord = &record
		}
		return core.ActionResult{Message: "switching to " + item.Target, Navigation: core.BackendTarget(item.Target)}, nil
	case core.ActionCreateWindow:
		return a.createWindow(ctx, request, item)
	case core.ActionRename:
		return a.renameItem(ctx, request, item)
	case core.ActionRenameSession:
		session, err := owningSessionItem(item)
		if err != nil {
			return core.ActionResult{}, err
		}
		return a.renameItem(ctx, request, session)
	case core.ActionDelete:
		return a.deleteItem(ctx, request, item)
	case core.ActionDeleteSession:
		session, err := owningSessionItem(item)
		if err != nil {
			return core.ActionResult{}, err
		}
		return a.deleteItem(ctx, request, session)
	default:
		return core.ActionResult{}, fmt.Errorf("action %q is not implemented", request.ActionID)
	}
}

func owningSessionItem(item liveItem) (liveItem, error) {
	if item.Kind != liveDestination || strings.TrimSpace(item.SessionTarget) == "" {
		return liveItem{}, fmt.Errorf("item %q has no owning session action", item.ID)
	}
	return liveItem{
		ID:             item.ID,
		SessionID:      item.SessionID,
		Kind:           liveSession,
		Label:          item.SessionName,
		Name:           item.SessionName,
		SessionName:    item.SessionName,
		Target:         item.SessionTarget,
		MutationTarget: item.SessionTarget,
		SessionTarget:  item.SessionTarget,
	}, nil
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

func (a *LiveAdapter) Navigate(ctx context.Context, navigation core.BackendTarget) error {
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
	scores, notice, err := a.loadFrecency(ctx)
	if err != nil {
		return list.Snapshot{}, err
	}
	snapshot, index, err := projectState(result, a.projection, scores, notice)
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

func (a *LiveAdapter) loadFrecency(ctx context.Context) (frecency.Scores, string, error) {
	if a.frecencyErr != nil {
		return frecency.Scores{}, "frecency unavailable: " + a.frecencyErr.Error(), nil
	}
	if a.frecency == nil {
		return frecency.Scores{}, "", nil
	}
	records, err := a.frecency.LoadContext(ctx)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return frecency.Scores{}, "", err
	}
	if err != nil {
		return frecency.Scores{}, "frecency state: " + err.Error(), nil
	}
	return frecency.NewScores(records, time.Now()), "", nil
}

func projectState(result backend.StateResult, projection projectionKind, scores frecency.Scores, notice string) (list.Snapshot, itemIndex, error) {
	homeDir, _ := os.UserHomeDir()
	var snapshot list.Snapshot
	var index itemIndex
	var err error
	if projection == projectionFlat {
		snapshot, index, err = projectFlatRanked(result, homeDir, scores)
	} else {
		snapshot, index, err = projectTree(result, homeDir)
	}
	if err != nil {
		return list.Snapshot{}, nil, fmt.Errorf("project live sessions: %w", err)
	}
	snapshot.Notice = notice
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
	scores, notice, err := a.loadFrecency(ctx)
	if err != nil {
		return core.ActionResult{}, err
	}
	snapshot, index, err := projectState(state, next, scores, notice)
	if err != nil {
		return core.ActionResult{}, err
	}
	preferred := preferredProjectionItem(selected, next, snapshot, index)
	a.projection = next
	a.index = index
	return core.ActionResult{Message: message, Snapshot: &snapshot, SelectItemID: preferred}, nil
}

func preferredProjectionItem(
	selected liveItem,
	projection projectionKind,
	snapshot list.Snapshot,
	index itemIndex,
) list.ItemID {
	if projection == projectionTree {
		return preferredTreeItem(selected, snapshot, index)
	}
	return preferredFlatItem(selected, snapshot, index)
}

func preferredTreeItem(selected liveItem, snapshot list.Snapshot, index itemIndex) list.ItemID {
	_, windowExists := index[selected.WindowID]
	if selected.WindowID != "" && windowExists {
		return selected.WindowID
	}
	_, sessionExists := index[selected.SessionID]
	if selected.SessionID != "" && sessionExists {
		return selected.SessionID
	}
	return projectionFallback(snapshot, index)
}

func preferredFlatItem(selected liveItem, snapshot list.Snapshot, index itemIndex) list.ItemID {
	var preferred list.ItemID
	if selected.WindowID != "" {
		preferred = firstMatchingItem(snapshot, index, func(item liveItem) bool {
			return item.WindowID == selected.WindowID && item.PaneActive
		})
	} else if selected.SessionID != "" {
		preferred = firstMatchingItem(snapshot, index, func(item liveItem) bool {
			return item.SessionID == selected.SessionID && item.WindowActive && item.PaneActive
		})
	}
	if preferred != "" {
		return preferred
	}
	return projectionFallback(snapshot, index)
}

func projectionFallback(snapshot list.Snapshot, index itemIndex) list.ItemID {
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
