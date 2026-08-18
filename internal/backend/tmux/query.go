package tmux

import (
	"context"
	"errors"
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

type queryParseError struct{ err error }

func (e *queryParseError) Error() string { return e.err.Error() }
func (e *queryParseError) Unwrap() error { return e.err }

func RunQuery[T any](ctx context.Context, c Client, q Query[T]) (T, error) {
	output, err := c.Run(ctx, q.Args()...)
	result, parseErr := q.Parse(output)
	if parseErr != nil {
		var zero T
		return zero, errors.Join(err, &queryParseError{err: parseErr})
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
		"-F", "#{session_id}|#{q:session_name}|#{window_id}|#{q:window_name}|#{window_index}|#{window_layout}|#{window_zoomed_flag}|#{window_active}|#{pane_id}|#{pane_index}|#{pane_active}|#{q:pane_current_path}|#{q:pane_current_command}|#{q:" + backend.WorkspacePathOption + "}",
	}
}

func (q LoadStateQuery) Parse(output string) (LoadStateResult, error) {
	currentID := getCurrentSessionID()
	builder := newStateBuilder()

	lines := strings.Split(output, "\n")
	// The single trailing blank line is the record terminator; any other
	// blank line means a value contained a newline.
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	if len(lines) < 2 {
		return LoadStateResult{}, fmt.Errorf("tmux state output is missing base indexes")
	}
	windowBaseIndex, err := parseIndex("window base", lines[0])
	if err != nil {
		return LoadStateResult{}, err
	}
	paneBaseIndex, err := parseIndex("pane base", lines[1])
	if err != nil {
		return LoadStateResult{}, err
	}
	result := LoadStateResult{WindowBaseIndex: windowBaseIndex, PaneBaseIndex: paneBaseIndex}

	for i, line := range lines[2:] {
		if line == "" {
			return LoadStateResult{}, fmt.Errorf("invalid tmux pane row %d: blank line means a value contained a newline", i+1)
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

const paneFieldCount = 14

func parsePaneLine(line string) (paneLine, error) {
	fields, err := parseFields(line, paneFieldCount)
	if err != nil {
		return paneLine{}, err
	}

	p := paneLine{
		sessionID:     fields[0],
		sessionName:   fields[1],
		windowID:      fields[2],
		windowName:    fields[3],
		windowLayout:  fields[5],
		paneID:        fields[8],
		panePath:      fields[11],
		paneCmd:       fields[12],
		workspacePath: fields[13],
	}
	if p.windowIndex, err = parseIndex("window", fields[4]); err != nil {
		return paneLine{}, err
	}
	if p.windowZoomed, err = parseFlag("window zoom", fields[6]); err != nil {
		return paneLine{}, err
	}
	if p.windowActive, err = parseFlag("window active", fields[7]); err != nil {
		return paneLine{}, err
	}
	if p.paneIndex, err = parseIndex("pane", fields[9]); err != nil {
		return paneLine{}, err
	}
	if p.paneActive, err = parseFlag("pane active", fields[10]); err != nil {
		return paneLine{}, err
	}
	if err := validateObjectID("session", p.sessionID, '$'); err != nil {
		return paneLine{}, err
	}
	if err := validateObjectID("window", p.windowID, '@'); err != nil {
		return paneLine{}, err
	}
	if err := validateObjectID("pane", p.paneID, '%'); err != nil {
		return paneLine{}, err
	}
	return p, nil
}

// parseFields splits one #{q:...}-escaped row: a backslash always
// introduces one literal byte, so an unescaped '|' is unambiguous.
func parseFields(line string, want int) ([]string, error) {
	fields := make([]string, 0, want)
	var value strings.Builder
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '\\':
			if i+1 == len(line) {
				return nil, fmt.Errorf("dangling escape in row %q", line)
			}
			i++
			value.WriteByte(line[i])
		case '|':
			fields = append(fields, value.String())
			value.Reset()
		default:
			value.WriteByte(line[i])
		}
	}
	fields = append(fields, value.String())
	if len(fields) != want {
		return nil, fmt.Errorf("expected %d fields, found %d", want, len(fields))
	}
	return fields, nil
}

func parseIndex(kind, value string) (int, error) {
	index, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || index < 0 {
		return 0, fmt.Errorf("invalid %s index %q", kind, value)
	}
	return index, nil
}

func validateObjectID(kind, value string, prefix byte) error {
	if len(value) < 2 || value[0] != prefix {
		return fmt.Errorf("invalid %s ID %q", kind, value)
	}
	number, err := strconv.Atoi(value[1:])
	if err != nil || number < 0 || strconv.Itoa(number) != value[1:] {
		return fmt.Errorf("invalid %s ID %q", kind, value)
	}
	return nil
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
	if sess.WorkspacePath == "" && p.workspacePath != "" {
		sess.WorkspacePath = p.workspacePath
	}
	win := b.getOrCreateWindow(sess, p.windowID, p.windowName, p.windowIndex, p.windowLayout, p.panePath)
	win.Active = p.windowActive
	win.Panes = append(win.Panes, Pane{ID: p.paneID, Index: p.paneIndex, Path: p.panePath, Command: p.paneCmd, Zoom: p.windowZoomed && p.paneActive, Active: p.paneActive})
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
