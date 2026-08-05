package tui

import (
	"context"
	"fmt"
	"os"
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

func (a *LiveAdapter) Refresh(ctx context.Context) (contracts.Snapshot, error) {
	return a.snapshotFromBackend(ctx)
}

func (a *LiveAdapter) Execute(ctx context.Context, intent contracts.Intent) (contracts.ActionResult, error) {
	b, err := a.detectBackend()
	if err != nil {
		return contracts.ActionResult{}, fmt.Errorf("failed to detect backend: %w", err)
	}

	switch intent.Type {
	case contracts.IntentSwitch:
		target := strings.TrimSpace(intent.Target)
		if target == "" {
			return contracts.ActionResult{}, fmt.Errorf("empty switch target")
		}
		if err := b.Switch(target); err != nil {
			return contracts.ActionResult{}, fmt.Errorf("switch to %q: %w", target, err)
		}
		return contracts.ActionResult{Message: "switched to " + target, NeedsRefresh: true}, nil
	case contracts.IntentCreateSession:
		name := payloadValue(intent.Payload, "name")
		if name == "" {
			return contracts.ActionResult{}, fmt.Errorf("session name is required")
		}

		actions := []backend.Action{backend.CreateSessionAction{Name: name}}
		workspacePath, err := activeWorkspacePath(b)
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

		if err := b.Apply(actions); err != nil {
			return contracts.ActionResult{}, fmt.Errorf("create session %q: %w", name, err)
		}

		message := "created session " + name
		if workspacePath != "" {
			message += " (workspace linked)"
		}
		return contracts.ActionResult{Message: message, NeedsRefresh: true}, nil
	case contracts.IntentCreateWindow:
		session := payloadValue(intent.Payload, "session")
		if session == "" {
			session = sessionFromTarget(intent.Target)
		}
		if session == "" {
			return contracts.ActionResult{}, fmt.Errorf("session target is required")
		}

		name := payloadValue(intent.Payload, "name")
		if name == "" {
			return contracts.ActionResult{}, fmt.Errorf("window name is required")
		}

		if err := b.Apply([]backend.Action{backend.CreateWindowAction{Session: session, Name: name}}); err != nil {
			return contracts.ActionResult{}, fmt.Errorf("create window %q in %q: %w", name, session, err)
		}
		return contracts.ActionResult{Message: "created window " + session + ":" + name, NeedsRefresh: true}, nil
	case contracts.IntentRenameSession:
		current := sessionFromTarget(intent.Target)
		if current == "" {
			return contracts.ActionResult{}, fmt.Errorf("session target is required")
		}
		name := payloadValue(intent.Payload, "name")
		if name == "" {
			return contracts.ActionResult{}, fmt.Errorf("new session name is required")
		}
		if err := b.Apply([]backend.Action{backend.RenameSessionAction{Current: current, New: name}}); err != nil {
			return contracts.ActionResult{}, fmt.Errorf("rename session %q to %q: %w", current, name, err)
		}
		return contracts.ActionResult{Message: "renamed session " + current + " -> " + name, NeedsRefresh: true}, nil
	case contracts.IntentRenameWindow:
		session, window, err := sessionWindowFromTarget(intent.Target)
		if err != nil {
			return contracts.ActionResult{}, err
		}
		name := payloadValue(intent.Payload, "name")
		if name == "" {
			return contracts.ActionResult{}, fmt.Errorf("new window name is required")
		}
		if err := b.Apply([]backend.Action{backend.RenameWindowAction{Session: session, Window: window, New: name}}); err != nil {
			return contracts.ActionResult{}, fmt.Errorf("rename window %q in %q to %q: %w", window, session, name, err)
		}
		return contracts.ActionResult{Message: "renamed window " + session + ":" + window + " -> " + name, NeedsRefresh: true}, nil
	case contracts.IntentDeleteSession:
		session := sessionFromTarget(intent.Target)
		if session == "" {
			return contracts.ActionResult{}, fmt.Errorf("session target is required")
		}
		if err := b.Apply([]backend.Action{backend.KillSessionAction{Name: session}}); err != nil {
			return contracts.ActionResult{}, fmt.Errorf("delete session %q: %w", session, err)
		}
		return contracts.ActionResult{Message: "deleted session " + session, NeedsRefresh: true}, nil
	case contracts.IntentDeleteWindow:
		session, window, err := sessionWindowFromTarget(intent.Target)
		if err != nil {
			return contracts.ActionResult{}, err
		}
		if err := b.Apply([]backend.Action{backend.KillWindowAction{Session: session, Window: window}}); err != nil {
			return contracts.ActionResult{}, fmt.Errorf("delete window %q in %q: %w", window, session, err)
		}
		return contracts.ActionResult{Message: "deleted window " + session + ":" + window, NeedsRefresh: true}, nil
	default:
		return contracts.ActionResult{}, fmt.Errorf("intent %q is not implemented yet", intent.Type)
	}
}

func (a *LiveAdapter) snapshotFromBackend(ctx context.Context) (contracts.Snapshot, error) {
	b, err := a.detectBackend()
	if err != nil {
		return contracts.Snapshot{}, fmt.Errorf("failed to detect backend: %w", err)
	}

	result, err := b.QueryState()
	if err != nil {
		return contracts.Snapshot{}, fmt.Errorf("failed to query sessions: %w", err)
	}

	activeTarget := buildActiveTarget(result.Active)
	activeWorkspace := workspacePathForSession(result.Sessions, result.Active.Session)
	if activeWorkspace == "" {
		activeWorkspace = "unmanaged"
	}
	snapshot := contracts.Snapshot{
		Nodes:        make([]contracts.Node, 0, len(result.Sessions)),
		ActiveNodeID: activeTarget,
		ContextBars: map[string]string{
			"source":    "live",
			"active":    activeTarget,
			"workspace": activeWorkspace,
		},
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
		sessionNode := contracts.Node{
			ID:     "session:" + sess.Name,
			Kind:   contracts.NodeKindSession,
			Label:  sess.Name,
			Target: sess.Name,
			Active: result.Active.Session == sess.Name,
		}

		for _, win := range sess.Windows {
			windowTarget := fmt.Sprintf("%s:%d", sess.Name, win.Index)
			windowLabel := fmt.Sprintf("%d", win.Index)
			if win.Name != "" {
				windowLabel = fmt.Sprintf("%d %s", win.Index, win.Name)
			}
			isActiveWindow := result.Active.Session == sess.Name && result.Active.WindowIndex == win.Index
			windowNode := contracts.Node{
				ID:       fmt.Sprintf("window:%s:%d", sess.Name, win.Index),
				ParentID: sessionNode.ID,
				Kind:     contracts.NodeKindWindow,
				Label:    windowLabel,
				Target:   windowTarget,
				Path:     displayPath(win.Path, homeDir),
				Active:   isActiveWindow,
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

func buildActiveTarget(active backend.ActiveContext) string {
	if active.Session == "" {
		return "none"
	}
	if active.Window == "" {
		return active.Session
	}
	windowRef := active.Window
	if active.WindowIndex >= 0 {
		windowRef = fmt.Sprintf("%d", active.WindowIndex)
	}
	if active.Pane >= 0 {
		return fmt.Sprintf("%s:%s.%d", active.Session, windowRef, active.Pane)
	}
	return fmt.Sprintf("%s:%s", active.Session, windowRef)
}

func workspacePathForSession(sessions []backend.Session, sessionName string) string {
	for _, sess := range sessions {
		if sess.Name == sessionName {
			return sess.WorkspacePath
		}
	}
	return ""
}

func activeWorkspacePath(b backend.Backend) (string, error) {
	state, err := b.QueryState()
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

func payloadValue(payload map[string]string, key string) string {
	if payload == nil {
		return ""
	}
	return strings.TrimSpace(payload[key])
}

func sessionFromTarget(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	session, _, hasWindow := strings.Cut(target, ":")
	if hasWindow {
		return strings.TrimSpace(session)
	}
	return target
}

func sessionWindowFromTarget(target string) (string, string, error) {
	target = strings.TrimSpace(target)
	session, rest, hasWindow := strings.Cut(target, ":")
	if !hasWindow || strings.TrimSpace(session) == "" || strings.TrimSpace(rest) == "" {
		return "", "", fmt.Errorf("window target must be in session:window format")
	}
	window, _, _ := strings.Cut(rest, ".")
	window = strings.TrimSpace(window)
	if window == "" {
		return "", "", fmt.Errorf("window target must include a window reference")
	}
	return strings.TrimSpace(session), window, nil
}
