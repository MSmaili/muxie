package tmux

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/MSmaili/hetki/internal/backend"
)

type TmuxBackend struct {
	client Client
}

func init() {
	backend.Register("tmux", func() (backend.Backend, error) {
		return NewBackend()
	})
}

func NewBackend() (*TmuxBackend, error) {
	c, err := New()
	if err != nil {
		return nil, err
	}
	return &TmuxBackend{client: c}, nil
}

func (b *TmuxBackend) Name() string {
	return "tmux"
}

func (b *TmuxBackend) QueryState() (backend.StateResult, error) {
	result, err := RunQuery(b.client, LoadStateQuery{})

	// tmux exits non-zero when list-panes runs against an empty server, after the
	// chained show-options calls have already emitted valid base indexes.
	if err != nil && !(len(result.Sessions) == 0 && isEmptyServerError(err)) {
		return backend.StateResult{}, err
	}

	sessions := make([]backend.Session, len(result.Sessions))
	for i, s := range result.Sessions {
		windows := make([]backend.Window, len(s.Windows))
		for j, w := range s.Windows {
			panes := make([]backend.Pane, len(w.Panes))
			for k, p := range w.Panes {
				panes[k] = backend.Pane{
					ID:      p.ID,
					Index:   p.Index,
					Path:    p.Path,
					Command: p.Command,
					Zoom:    p.Zoom,
				}
			}
			windows[j] = backend.Window{
				ID:     w.ID,
				Name:   w.Name,
				Index:  w.Index,
				Path:   w.Path,
				Layout: w.Layout,
				Panes:  panes,
			}
		}
		sessions[i] = backend.Session{
			ID:            s.ID,
			Name:          s.Name,
			WorkspacePath: s.WorkspacePath,
			Windows:       windows,
		}
	}

	return backend.StateResult{
		Sessions: sessions,
		Active: backend.ActiveContext{
			SessionID:   result.Active.SessionID,
			Session:     result.Active.Session,
			WindowID:    result.Active.WindowID,
			Window:      result.Active.Window,
			WindowIndex: result.Active.WindowIndex,
			PaneID:      result.Active.PaneID,
			Pane:        result.Active.Pane,
			Path:        result.Active.Path,
		},
	}, nil
}

func (b *TmuxBackend) Apply(actions []backend.Action) error {
	created := make(map[string]*createdTarget)
	for i, action := range actions {
		if action == nil {
			return fmt.Errorf("action %d is nil", i)
		}
		if err := action.Validate(); err != nil {
			return fmt.Errorf("action %d is invalid: %w", i, err)
		}
		if err := b.applyAction(action, created); err != nil {
			return fmt.Errorf("action %d failed: %w", i, err)
		}
	}
	return nil
}

func (b *TmuxBackend) DryRun(actions []backend.Action) []string {
	args := dryRunArgs(actions)
	lines := make([]string, len(args))
	for i := range args {
		lines[i] = "tmux " + strings.Join(args[i], " ")
	}
	return lines
}

func (b *TmuxBackend) Attach(session string) error {
	return b.switchTo(session)
}

func (b *TmuxBackend) Switch(target string) error {
	session, rest, hasWindow := strings.Cut(target, ":")
	if !hasWindow {
		return b.switchTo(target)
	}

	state, err := RunQuery(b.client, LoadStateQuery{})
	if err != nil {
		return err
	}

	window, paneStr, hasPane := strings.Cut(rest, ".")

	winIndex, err := resolveWindowIndex(state.Sessions, session, window)
	if err != nil {
		return err
	}

	resolved := fmt.Sprintf("%s:%d", session, winIndex)
	if hasPane {
		pane, err := strconv.Atoi(strings.TrimSpace(paneStr))
		if err != nil || pane < 0 {
			return fmt.Errorf("invalid pane index %q", paneStr)
		}
		resolved = fmt.Sprintf("%s.%d", resolved, pane+state.PaneBaseIndex)
	}

	return b.switchTo(resolved)
}

func (b *TmuxBackend) switchTo(target string) error {
	if isInsideTmux() {
		return b.client.Execute(SwitchClient{Target: target})
	}
	return b.client.Execute(AttachSession{Target: target})
}

func isInsideTmux() bool {
	return os.Getenv("TMUX") != ""
}

func isEmptyServerError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no server running") || strings.Contains(message, "no current target")
}

func findWindowIndex(sessions []Session, sessionName, windowName string) (int, error) {
	for _, s := range sessions {
		if s.Name != sessionName {
			continue
		}
		index := -1
		for _, w := range s.Windows {
			if w.Name != windowName {
				continue
			}
			if index >= 0 {
				return 0, fmt.Errorf("window name %q is ambiguous in session %q", windowName, sessionName)
			}
			index = w.Index
		}
		if index >= 0 {
			return index, nil
		}
		return 0, fmt.Errorf("window %q not found in session %q", windowName, sessionName)
	}
	return 0, fmt.Errorf("session %q not found", sessionName)
}

func resolveWindowIndex(sessions []Session, sessionName, window string) (int, error) {
	window = strings.TrimSpace(window)
	if strings.HasPrefix(window, "@") {
		for _, session := range sessions {
			if session.Name != sessionName {
				continue
			}
			for _, candidate := range session.Windows {
				if candidate.ID == window {
					return candidate.Index, nil
				}
			}
			return 0, fmt.Errorf("window ID %q not found in session %q", window, sessionName)
		}
		return 0, fmt.Errorf("session %q not found", sessionName)
	}
	if idx, err := strconv.Atoi(window); err == nil {
		if windowIndexExists(sessions, sessionName, idx) {
			return idx, nil
		}
		return 0, fmt.Errorf("window index %d not found in session %q", idx, sessionName)
	}
	return findWindowIndex(sessions, sessionName, window)
}

func windowIndexExists(sessions []Session, sessionName string, index int) bool {
	for _, s := range sessions {
		if s.Name != sessionName {
			continue
		}
		for _, w := range s.Windows {
			if w.Index == index {
				return true
			}
		}
		return false
	}
	return false
}

type createdTarget struct {
	name     string
	windowID string
	paneIDs  []string
}

func (b *TmuxBackend) applyAction(a backend.Action, created map[string]*createdTarget) error {
	switch action := a.(type) {
	case backend.CreateSessionAction:
		target, err := b.createTarget(CreateSession{Name: action.Name, WindowName: action.WindowName, Path: action.Path}, action.WindowName)
		if err == nil {
			created[action.Name] = target
		}
		return err
	case backend.CreateWindowAction:
		target, err := b.createTarget(CreateWindow{Session: action.Session, Name: action.Name, Path: action.Path}, action.Name)
		if err == nil {
			created[action.Session] = target
		}
		return err
	case backend.SplitPaneAction:
		target, err := followupTarget(created, action.Session, action.Window)
		if err != nil {
			return err
		}
		output, err := b.client.Run(printArgs(SplitPane{Target: target.windowID, Path: action.Path}, "#{pane_id}")...)
		if err != nil {
			return err
		}
		paneID := strings.TrimSpace(output)
		if !strings.HasPrefix(paneID, "%") {
			return fmt.Errorf("tmux returned invalid pane ID %q", paneID)
		}
		target.paneIDs = append(target.paneIDs, paneID)
		return nil
	case backend.SendKeysAction:
		paneID, err := followupPaneID(created, action.Session, action.Window, action.Pane)
		if err != nil {
			return err
		}
		return b.client.Execute(SendKeys{Target: paneID, Keys: action.Command})
	case backend.SelectLayoutAction:
		target, err := followupTarget(created, action.Session, action.Window)
		if err != nil {
			return err
		}
		return b.client.Execute(SelectLayout{Target: target.windowID, Layout: action.Layout})
	case backend.ZoomPaneAction:
		paneID, err := followupPaneID(created, action.Session, action.Window, action.Pane)
		if err != nil {
			return err
		}
		return b.client.Execute(ZoomPane{Target: paneID})
	default:
		tmuxAction := staticAction(a)
		if tmuxAction == nil {
			return fmt.Errorf("unsupported backend action %T", a)
		}
		return b.client.Execute(tmuxAction)
	}
}

func (b *TmuxBackend) createTarget(action Action, name string) (*createdTarget, error) {
	output, err := b.client.Run(printArgs(action, "#{window_id}|#{pane_id}")...)
	if err != nil {
		return nil, err
	}
	windowID, paneID, ok := strings.Cut(strings.TrimSpace(output), "|")
	if !ok || !strings.HasPrefix(windowID, "@") || !strings.HasPrefix(paneID, "%") {
		return nil, fmt.Errorf("tmux returned invalid created target %q", output)
	}
	return &createdTarget{name: name, windowID: windowID, paneIDs: []string{paneID}}, nil
}

func followupTarget(created map[string]*createdTarget, session, window string) (*createdTarget, error) {
	target := created[session]
	if target == nil || target.name != window {
		return nil, fmt.Errorf("created window identity unavailable for %s:%s", session, window)
	}
	return target, nil
}

func followupPaneID(created map[string]*createdTarget, session, window string, pane int) (string, error) {
	target, err := followupTarget(created, session, window)
	if err != nil {
		return "", err
	}
	if pane < 0 || pane >= len(target.paneIDs) {
		return "", fmt.Errorf("created pane %d unavailable for %s:%s", pane, session, window)
	}
	return target.paneIDs[pane], nil
}

func printArgs(action Action, format string) []string {
	return append(action.Args(), "-P", "-F", format)
}

func staticAction(a backend.Action) Action {
	switch action := a.(type) {
	case backend.RenameSessionAction:
		return RenameSession{Target: action.Current, Name: action.New}
	case backend.RenameWindowAction:
		return RenameWindow{Target: action.WindowID, Name: action.New}
	case backend.KillSessionAction:
		return KillSession{Name: action.Name}
	case backend.KillWindowAction:
		return KillWindow{Target: action.WindowID}
	case backend.SetSessionOptionAction:
		return SetSessionOption{Session: action.Session, Key: action.Key, Value: action.Value}
	default:
		return nil
	}
}

func dryRunArgs(actions []backend.Action) [][]string {
	result := make([][]string, 0, len(actions))
	created := make(map[string]*createdTarget)
	for _, a := range actions {
		switch action := a.(type) {
		case backend.CreateSessionAction:
			result = append(result, printArgs(CreateSession{Name: action.Name, WindowName: action.WindowName, Path: action.Path}, "#{window_id}|#{pane_id}"))
			created[action.Name] = dryRunTarget(action.Name, action.WindowName)
		case backend.CreateWindowAction:
			result = append(result, printArgs(CreateWindow{Session: action.Session, Name: action.Name, Path: action.Path}, "#{window_id}|#{pane_id}"))
			created[action.Session] = dryRunTarget(action.Session, action.Name)
		case backend.SplitPaneAction:
			target, err := followupTarget(created, action.Session, action.Window)
			if err != nil {
				continue
			}
			result = append(result, printArgs(SplitPane{Target: target.windowID, Path: action.Path}, "#{pane_id}"))
			target.paneIDs = append(target.paneIDs, fmt.Sprintf("<new-pane:%s:%s:%d>", action.Session, action.Window, len(target.paneIDs)))
		case backend.SendKeysAction:
			if paneID, err := followupPaneID(created, action.Session, action.Window, action.Pane); err == nil {
				result = append(result, SendKeys{Target: paneID, Keys: action.Command}.Args())
			}
		case backend.SelectLayoutAction:
			if target, err := followupTarget(created, action.Session, action.Window); err == nil {
				result = append(result, SelectLayout{Target: target.windowID, Layout: action.Layout}.Args())
			}
		case backend.ZoomPaneAction:
			if paneID, err := followupPaneID(created, action.Session, action.Window, action.Pane); err == nil {
				result = append(result, ZoomPane{Target: paneID}.Args())
			}
		default:
			if tmuxAction := staticAction(a); tmuxAction != nil {
				result = append(result, tmuxAction.Args())
			}
		}
	}
	return result
}

func dryRunTarget(session, window string) *createdTarget {
	return &createdTarget{
		name:     window,
		windowID: fmt.Sprintf("<new-window:%s:%s>", session, window),
		paneIDs:  []string{fmt.Sprintf("<new-pane:%s:%s:0>", session, window)},
	}
}
