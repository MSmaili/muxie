package tmux

import (
	"fmt"
	"os"
	"strings"
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
	Name    string
	Windows []Window
}

type LoadStateResult struct {
	Sessions        []Session
	Active          ActiveContext
	PaneBaseIndex   int
	WindowBaseIndex int
}

type ActiveContext struct {
	Session string
	Window  string
	Pane    int
	Path    string
}

type LoadStateQuery struct{}

func (q LoadStateQuery) Args() []string {
	return []string{
		"start-server",
		";", "show-options", "-gv", "base-index",
		";", "show-options", "-gv", "pane-base-index",
		";", "list-panes", "-a",
		"-F", "#{session_id}|#{session_name}|#{window_name}|#{window_index}|#{window_active}|#{pane_index}|#{pane_active}|#{pane_current_path}|#{pane_current_command}",
	}
}

func (q LoadStateQuery) Parse(output string) (LoadStateResult, error) {
	if output == "" {
		return LoadStateResult{}, nil
	}

	currentID := getCurrentSessionID()
	builder := newStateBuilder()

	lines := strings.Split(output, "\n")

	var result LoadStateResult
	if len(lines) >= 2 {
		fmt.Sscanf(strings.TrimSpace(lines[0]), "%d", &result.WindowBaseIndex)
		fmt.Sscanf(strings.TrimSpace(lines[1]), "%d", &result.PaneBaseIndex)
	}

	for _, line := range lines[2:] {
		if p, ok := parsePaneLine(line); ok {
			builder.addPane(p, currentID)
		}
	}

	built := builder.result()
	result.Sessions = built.Sessions
	result.Active = built.Active

	return result, nil
}

type paneLine struct {
	sessionID, sessionName, windowName string
	windowIndex                        int
	windowActive                       bool
	paneIndex                          int
	paneActive                         bool
	panePath, paneCmd                  string
}

func parsePaneLine(line string) (paneLine, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return paneLine{}, false
	}

	var p paneLine
	var ok bool
	var windowIndexStr, windowActiveStr, paneIndexStr, paneActiveStr string

	if p.sessionID, line, ok = strings.Cut(line, "|"); !ok {
		return paneLine{}, false
	}
	if p.sessionName, line, ok = strings.Cut(line, "|"); !ok {
		return paneLine{}, false
	}
	if p.windowName, line, ok = strings.Cut(line, "|"); !ok {
		return paneLine{}, false
	}
	if windowIndexStr, line, ok = strings.Cut(line, "|"); !ok {
		return paneLine{}, false
	}
	fmt.Sscanf(windowIndexStr, "%d", &p.windowIndex)
	if windowActiveStr, line, ok = strings.Cut(line, "|"); !ok {
		return paneLine{}, false
	}
	p.windowActive = windowActiveStr == "1"

	if paneIndexStr, line, ok = strings.Cut(line, "|"); !ok {
		return paneLine{}, false
	}
	fmt.Sscanf(paneIndexStr, "%d", &p.paneIndex)

	if paneActiveStr, line, ok = strings.Cut(line, "|"); !ok {
		return paneLine{}, false
	}
	p.paneActive = paneActiveStr == "1"

	if p.panePath, line, ok = strings.Cut(line, "|"); !ok {
		return paneLine{}, false
	}
	p.paneCmd = line
	return p, true
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
		b.active.Session = p.sessionName
		if p.windowActive {
			b.active.Window = p.windowName
		}
		if p.paneActive {
			b.active.Pane = p.paneIndex
			b.active.Path = p.panePath
		}
	}

	sess := b.getOrCreateSession(p.sessionName)
	win := b.getOrCreateWindow(sess, p.windowName, p.windowIndex, p.panePath)
	win.Panes = append(win.Panes, Pane{Path: p.panePath, Command: p.paneCmd})
}

func (b *stateBuilder) getOrCreateSession(name string) *Session {
	if sess, ok := b.sessions[name]; ok {
		return sess
	}
	sess := &Session{Name: name}
	b.sessions[name] = sess
	return sess
}

func (b *stateBuilder) getOrCreateWindow(sess *Session, name string, index int, path string) *Window {
	for i := range sess.Windows {
		if sess.Windows[i].Index == index {
			return &sess.Windows[i]
		}
	}
	sess.Windows = append(sess.Windows, Window{Name: name, Index: index, Path: path})
	return &sess.Windows[len(sess.Windows)-1]
}

func (b *stateBuilder) result() LoadStateResult {
	sessions := make([]Session, 0, len(b.sessions))
	for _, s := range b.sessions {
		sessions = append(sessions, *s)
	}
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
