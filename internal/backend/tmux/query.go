package tmux

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/MSmaili/hetki/internal/backend"
)

type Query[T any] interface {
	Args() []string
	Parse(output string) (T, error)
}

func RunQuery[T any](c Client, q Query[T]) (T, error) {
	output, err := c.Run(q.Args()...)
	result, parseErr := q.Parse(output)
	if parseErr != nil {
		var zero T
		return zero, parseErr
	}
	return result, err
}

type Session struct {
	ID            string
	Name          string
	WorkspacePath string
	Windows       []Window
}

type LoadStateResult struct {
	Sessions        []Session
	Active          ActiveContext
	PaneBaseIndex   int
	WindowBaseIndex int
}

type ActiveContext struct {
	SessionID   string
	Session     string
	WindowID    string
	Window      string
	WindowIndex int
	PaneID      string
	Pane        int
	Path        string
}

type LoadStateQuery struct{}

func (q LoadStateQuery) Args() []string {
	return []string{
		"start-server",
		";", "show-options", "-gv", "base-index",
		";", "show-options", "-gv", "pane-base-index",
		";", "list-panes", "-a",
		"-F", "#{session_id}|#{session_name}|#{window_id}|#{window_name}|#{window_index}|#{window_layout}|#{window_zoomed_flag}|#{window_active}|#{pane_id}|#{pane_index}|#{pane_active}|#{pane_current_path}|#{pane_current_command}|#{" + backend.WorkspacePathOption + "}",
	}
}

func (q LoadStateQuery) Parse(output string) (LoadStateResult, error) {
	if output == "" {
		return LoadStateResult{}, nil
	}

	currentID := getCurrentSessionID()
	builder := newStateBuilder()

	lines := strings.Split(output, "\n")

	if len(lines) < 2 {
		return LoadStateResult{}, fmt.Errorf("tmux state output is missing base indexes")
	}
	windowBaseIndex, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil {
		return LoadStateResult{}, fmt.Errorf("invalid window base index %q: %w", lines[0], err)
	}
	paneBaseIndex, err := strconv.Atoi(strings.TrimSpace(lines[1]))
	if err != nil {
		return LoadStateResult{}, fmt.Errorf("invalid pane base index %q: %w", lines[1], err)
	}
	result := LoadStateResult{WindowBaseIndex: windowBaseIndex, PaneBaseIndex: paneBaseIndex}

	for i, line := range lines[2:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		p, err := parsePaneLine(line)
		if err != nil {
			return LoadStateResult{}, fmt.Errorf("invalid tmux pane row %d: %w", i+1, err)
		}
		builder.addPane(p, currentID)
	}

	built := builder.result()
	result.Sessions = built.Sessions
	result.Active = built.Active

	return result, nil
}

type paneLine struct {
	sessionID, sessionName, windowID, windowName string
	windowIndex                                  int
	windowLayout                                 string
	windowZoomed                                 bool
	windowActive                                 bool
	paneID                                       string
	paneIndex                                    int
	paneActive                                   bool
	panePath, paneCmd                            string
	workspacePath                                string
}

func parsePaneLine(line string) (paneLine, error) {
	line = strings.TrimSpace(line)
	cut := func(field string) (string, error) {
		value, rest, ok := strings.Cut(line, "|")
		if !ok {
			return "", fmt.Errorf("missing %s", field)
		}
		line = rest
		return value, nil
	}

	var p paneLine
	var err error
	if p.sessionID, err = cut("session ID"); err != nil {
		return paneLine{}, err
	}
	if p.sessionName, err = cut("session name"); err != nil {
		return paneLine{}, err
	}
	if p.windowID, err = cut("window ID"); err != nil {
		return paneLine{}, err
	}
	if p.windowName, err = cut("window name"); err != nil {
		return paneLine{}, err
	}
	windowIndex, err := cut("window index")
	if err != nil {
		return paneLine{}, err
	}
	if p.windowIndex, err = parseIndex("window", windowIndex); err != nil {
		return paneLine{}, err
	}
	if p.windowLayout, err = cut("window layout"); err != nil {
		return paneLine{}, err
	}
	windowZoomed, err := cut("window zoom flag")
	if err != nil {
		return paneLine{}, err
	}
	if p.windowZoomed, err = parseFlag("window zoom", windowZoomed); err != nil {
		return paneLine{}, err
	}
	windowActive, err := cut("window active flag")
	if err != nil {
		return paneLine{}, err
	}
	if p.windowActive, err = parseFlag("window active", windowActive); err != nil {
		return paneLine{}, err
	}
	if p.paneID, err = cut("pane ID"); err != nil {
		return paneLine{}, err
	}
	paneIndex, err := cut("pane index")
	if err != nil {
		return paneLine{}, err
	}
	if p.paneIndex, err = parseIndex("pane", paneIndex); err != nil {
		return paneLine{}, err
	}
	paneActive, err := cut("pane active flag")
	if err != nil {
		return paneLine{}, err
	}
	if p.paneActive, err = parseFlag("pane active", paneActive); err != nil {
		return paneLine{}, err
	}
	if p.panePath, err = cut("pane path"); err != nil {
		return paneLine{}, err
	}
	if !strings.HasPrefix(p.sessionID, "$") || !strings.HasPrefix(p.windowID, "@") || !strings.HasPrefix(p.paneID, "%") {
		return paneLine{}, fmt.Errorf("invalid object IDs %q, %q, %q", p.sessionID, p.windowID, p.paneID)
	}
	if idx := strings.LastIndex(line, "|"); idx >= 0 {
		p.paneCmd = line[:idx]
		p.workspacePath = line[idx+1:]
	} else {
		p.paneCmd = line
	}
	return p, nil
}

func parseIndex(kind, value string) (int, error) {
	index, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || index < 0 {
		return 0, fmt.Errorf("invalid %s index %q", kind, value)
	}
	return index, nil
}

func parseFlag(kind, value string) (bool, error) {
	switch value {
	case "0":
		return false, nil
	case "1":
		return true, nil
	default:
		return false, fmt.Errorf("invalid %s flag %q", kind, value)
	}
}

type stateBuilder struct {
	sessions map[string]*Session
	active   ActiveContext
}

func newStateBuilder() *stateBuilder {
	return &stateBuilder{sessions: make(map[string]*Session)}
}

func (b *stateBuilder) addPane(p paneLine, currentID string) {
	if p.sessionID == currentID {
		b.active.SessionID = p.sessionID
		b.active.Session = p.sessionName
		if p.windowActive {
			b.active.WindowID = p.windowID
			b.active.Window = p.windowName
			b.active.WindowIndex = p.windowIndex
		}
		if p.windowActive && p.paneActive {
			b.active.PaneID = p.paneID
			b.active.Pane = p.paneIndex
			b.active.Path = p.panePath
		}
	}

	sess := b.getOrCreateSession(p.sessionID, p.sessionName)
	if sess.WorkspacePath == "" && strings.TrimSpace(p.workspacePath) != "" {
		sess.WorkspacePath = strings.TrimSpace(p.workspacePath)
	}
	win := b.getOrCreateWindow(sess, p.windowID, p.windowName, p.windowIndex, p.windowLayout, p.panePath)
	win.Panes = append(win.Panes, Pane{ID: p.paneID, Index: p.paneIndex, Path: p.panePath, Command: p.paneCmd, Zoom: p.windowZoomed && p.paneActive})
}

func (b *stateBuilder) getOrCreateSession(id, name string) *Session {
	if sess, ok := b.sessions[id]; ok {
		return sess
	}
	sess := &Session{ID: id, Name: name}
	b.sessions[id] = sess
	return sess
}

func (b *stateBuilder) getOrCreateWindow(sess *Session, id, name string, index int, layout, path string) *Window {
	for i := range sess.Windows {
		if sess.Windows[i].ID == id {
			if layout != "" {
				sess.Windows[i].Layout = layout
			}
			return &sess.Windows[i]
		}
	}
	sess.Windows = append(sess.Windows, Window{ID: id, Name: name, Index: index, Path: path, Layout: layout})
	return &sess.Windows[len(sess.Windows)-1]
}

func (b *stateBuilder) result() LoadStateResult {
	sessions := make([]Session, 0, len(b.sessions))
	for _, session := range b.sessions {
		copy := *session
		sort.Slice(copy.Windows, func(i, j int) bool {
			return copy.Windows[i].Index < copy.Windows[j].Index
		})
		sessions = append(sessions, copy)
	}
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].Name == sessions[j].Name {
			return sessions[i].ID < sessions[j].ID
		}
		return sessions[i].Name < sessions[j].Name
	})
	return LoadStateResult{Sessions: sessions, Active: b.active}
}

func getCurrentSessionID() string {
	tmuxEnv := os.Getenv("TMUX")
	if tmuxEnv == "" {
		return ""
	}
	parts := strings.Split(tmuxEnv, ",")
	if len(parts) < 3 {
		return ""
	}
	return "$" + parts[2]
}
