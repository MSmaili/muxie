package tui

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/MSmaili/hetki/internal/backend"
	"github.com/MSmaili/hetki/internal/tui/contracts"
)

type LiveAdapter struct {
	DetectBackend func(...string) (backend.Backend, error)
	cached        backend.Backend
}

func NewLiveAdapter(detectBackend func(...string) (backend.Backend, error)) *LiveAdapter {
	return &LiveAdapter{DetectBackend: detectBackend}
}

func (a *LiveAdapter) Load(ctx context.Context) (contracts.Snapshot, error) {
	return a.snapshotFromBackend(ctx)
}

func (a *LiveAdapter) Execute(ctx context.Context, intent contracts.Intent) (contracts.ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return contracts.ActionResult{}, err
	}
	b, err := a.detectBackend()
	if err != nil {
		return contracts.ActionResult{}, fmt.Errorf("failed to detect backend: %w", err)
	}

	switch intent.Type {
	case contracts.IntentSwitch:
		target := strings.TrimSpace(string(intent.Target))
		if target == "" {
			return contracts.ActionResult{}, fmt.Errorf("empty switch target")
		}
		return contracts.ActionResult{
			Message:    "switching to " + target,
			Navigation: contracts.BackendTarget(target),
		}, nil
	case contracts.IntentCreateSession:
		name := strings.TrimSpace(intent.Name)
		if name == "" {
			return contracts.ActionResult{}, fmt.Errorf("session name is required")
		}

		actions := []backend.Action{backend.CreateSessionAction{Name: name}}
		workspacePath, err := activeWorkspacePath(ctx, b)
		if err != nil {
			return contracts.ActionResult{}, err
		}
		if workspacePath != "" {
			actions = append(actions, backend.SetSessionOptionAction{
				Session: name,
				Key:     backend.WorkspacePathOption,
				Value:   workspacePath,
			})
		}

		if err := b.Apply(ctx, actions); err != nil {
			return contracts.ActionResult{}, fmt.Errorf("create session %q: %w", name, err)
		}

		message := "created session " + name
		if workspacePath != "" {
			message += " (workspace linked)"
		}
		return contracts.ActionResult{Message: message, NeedsRefresh: true}, nil
	case contracts.IntentCreateWindow:
		session := strings.TrimSpace(string(intent.Session))
		if session == "" {
			session = sessionFromTarget(intent.Target)
		}
		if session == "" {
			return contracts.ActionResult{}, fmt.Errorf("session target is required")
		}

		name := strings.TrimSpace(intent.Name)
		if name == "" {
			return contracts.ActionResult{}, fmt.Errorf("window name is required")
		}

		if err := b.Apply(ctx, []backend.Action{backend.CreateWindowAction{Session: session, Name: name}}); err != nil {
			return contracts.ActionResult{}, fmt.Errorf("create window %q in %q: %w", name, session, err)
		}
		return contracts.ActionResult{Message: "created window " + session + ":" + name, NeedsRefresh: true}, nil
	case contracts.IntentRenameSession:
		current := sessionFromTarget(intent.Target)
		if current == "" {
			return contracts.ActionResult{}, fmt.Errorf("session target is required")
		}
		name := strings.TrimSpace(intent.Name)
		if name == "" {
			return contracts.ActionResult{}, fmt.Errorf("new session name is required")
		}
		if err := b.Apply(ctx, []backend.Action{backend.RenameSessionAction{Current: current, New: name}}); err != nil {
			return contracts.ActionResult{}, fmt.Errorf("rename session %q to %q: %w", current, name, err)
		}
		return contracts.ActionResult{Message: "renamed session " + current + " -> " + name, NeedsRefresh: true}, nil
	case contracts.IntentRenameWindow:
		session, window, err := sessionWindowFromTarget(intent.Target)
		if err != nil {
			return contracts.ActionResult{}, err
		}
		name := strings.TrimSpace(intent.Name)
		if name == "" {
			return contracts.ActionResult{}, fmt.Errorf("new window name is required")
		}
		if err := b.Apply(ctx, []backend.Action{backend.RenameWindowAction{Session: session, Window: window, WindowID: window, New: name}}); err != nil {
			return contracts.ActionResult{}, fmt.Errorf("rename window %q in %q to %q: %w", window, session, name, err)
		}
		return contracts.ActionResult{Message: "renamed window " + session + ":" + window + " -> " + name, NeedsRefresh: true}, nil
	case contracts.IntentDeleteSession:
		session := sessionFromTarget(intent.Target)
		if session == "" {
			return contracts.ActionResult{}, fmt.Errorf("session target is required")
		}
		if err := b.Apply(ctx, []backend.Action{backend.KillSessionAction{Name: session}}); err != nil {
			return contracts.ActionResult{}, fmt.Errorf("delete session %q: %w", session, err)
		}
		return contracts.ActionResult{Message: "deleted session " + session, NeedsRefresh: true}, nil
	case contracts.IntentDeleteWindow:
		session, window, err := sessionWindowFromTarget(intent.Target)
		if err != nil {
			return contracts.ActionResult{}, err
		}
		if err := b.Apply(ctx, []backend.Action{backend.KillWindowAction{Session: session, Window: window, WindowID: window}}); err != nil {
			return contracts.ActionResult{}, fmt.Errorf("delete window %q in %q: %w", window, session, err)
		}
		return contracts.ActionResult{Message: "deleted window " + session + ":" + window, NeedsRefresh: true}, nil
	default:
		return contracts.ActionResult{}, fmt.Errorf("intent %q is not implemented yet", intent.Type)
	}
}

func (a *LiveAdapter) Navigate(ctx context.Context, navigation contracts.BackendTarget) error {
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

func (a *LiveAdapter) snapshotFromBackend(ctx context.Context) (contracts.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return contracts.Snapshot{}, err
	}
	b, err := a.detectBackend()
	if err != nil {
		return contracts.Snapshot{}, fmt.Errorf("failed to detect backend: %w", err)
	}

	result, err := b.QueryState(ctx)
	if err != nil {
		return contracts.Snapshot{}, fmt.Errorf("failed to query sessions: %w", err)
	}

	activeWorkspace := workspacePathForSession(result.Sessions, result.Active.Session)
	if activeWorkspace == "" {
		activeWorkspace = "unmanaged"
	}
	snapshot := contracts.Snapshot{
		Nodes:     make([]contracts.Node, 0, len(result.Sessions)),
		Workspace: activeWorkspace,
		Capabilities: map[contracts.Capability]bool{
			contracts.CapabilityRefresh:       true,
			contracts.CapabilitySwitch:        true,
			contracts.CapabilityCreateSession: true,
			contracts.CapabilityCreateWindow:  true,
			contracts.CapabilityRenameSession: true,
			contracts.CapabilityRenameWindow:  true,
			contracts.CapabilityDeleteSession: true,
			contracts.CapabilityDeleteWindow:  true,
		},
	}

	homeDir, _ := os.UserHomeDir()

	for _, sess := range result.Sessions {
		sessionRef := sess.ID
		if sessionRef == "" {
			sessionRef = sess.Name
		}
		// Stable refs: names may contain ':' or '.'.
		sessionNode := contracts.Node{
			ID:     contracts.NodeID("session:" + sessionRef),
			Kind:   contracts.NodeKindSession,
			Label:  sess.Name,
			Name:   sess.Name,
			Target: contracts.BackendTarget(sessionRef),
			Active: result.Active.Session == sess.Name,
		}
		if sessionNode.Active {
			snapshot.ActiveNodeID = sessionNode.ID
		}

		for _, win := range sess.Windows {
			windowRef := win.ID
			windowNodeID := contracts.NodeID("window:" + windowRef)
			if windowRef == "" {
				windowRef = fmt.Sprintf("%d", win.Index)
				windowNodeID = contracts.NodeID(fmt.Sprintf("window:%s:%s", sess.Name, windowRef))
			}
			windowTarget := fmt.Sprintf("%s:%s", sessionRef, windowRef)
			windowLabel := fmt.Sprintf("%d", win.Index)
			if win.Name != "" {
				windowLabel = fmt.Sprintf("%d %s", win.Index, win.Name)
			}
			isActiveWindow := result.Active.Session == sess.Name && result.Active.WindowIndex == win.Index
			windowNode := contracts.Node{
				ID:       windowNodeID,
				ParentID: sessionNode.ID,
				Kind:     contracts.NodeKindWindow,
				Label:    windowLabel,
				Name:     win.Name,
				Target:   contracts.BackendTarget(windowTarget),
				Path:     displayPath(win.Path, homeDir),
				Active:   isActiveWindow,
			}
			if windowNode.Active {
				snapshot.ActiveNodeID = windowNode.ID
			}

			sessionNode.Children = append(sessionNode.Children, windowNode)
		}

		snapshot.Nodes = append(snapshot.Nodes, sessionNode)
	}

	return snapshot, nil
}

func (a *LiveAdapter) detectBackend() (backend.Backend, error) {
	if a.cached != nil {
		return a.cached, nil
	}
	var (
		b   backend.Backend
		err error
	)
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
	for _, sess := range sessions {
		if sess.Name == sessionName {
			return sess.WorkspacePath
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

// displayPath collapses the user's home directory to "~" for compact display.
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

func sessionFromTarget(target contracts.BackendTarget) string {
	value := strings.TrimSpace(string(target))
	if value == "" {
		return ""
	}
	session, _, hasWindow := strings.Cut(value, ":")
	if hasWindow {
		return strings.TrimSpace(session)
	}
	return value
}

func sessionWindowFromTarget(target contracts.BackendTarget) (string, string, error) {
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
