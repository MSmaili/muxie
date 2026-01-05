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

type Session struct {
	Name          string
	WorkspacePath string
	Windows       []Window
}

type LoadStateResult struct {
	Sessions       []Session
	CurrentSession string
}

type LoadStateQuery struct{}

func (q LoadStateQuery) Args() []string {
	return []string{"list-panes", "-a", "-F", "#{session_id}|#{session_name}|#{window_name}|#{pane_current_path}|#{pane_current_command}|#{TMS_WORKSPACE_PATH}"}
}

func (q LoadStateQuery) Parse(output string) (LoadStateResult, error) {
	result := LoadStateResult{}
	currentSessionID := getCurrentSessionID()

	if output == "" {
		return result, nil
	}

	sessionMap := make(map[string]*Session)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 6 {
			continue
		}

		sessID, sessName, winName, panePath, paneCmd, workspacePath := parts[0], parts[1], parts[2], parts[3], parts[4], parts[5]

		if sessID == currentSessionID {
			result.CurrentSession = sessName
		}

		session, ok := sessionMap[sessName]
		if !ok {
			session = &Session{Name: sessName, WorkspacePath: workspacePath}
			sessionMap[sessName] = session
		}

		var window *Window
		for i := range session.Windows {
			if session.Windows[i].Name == winName {
				window = &session.Windows[i]
				break
			}
		}
		if window == nil {
			session.Windows = append(session.Windows, Window{Name: winName, Path: panePath})
			window = &session.Windows[len(session.Windows)-1]
		}

		window.Panes = append(window.Panes, Pane{Path: panePath, Command: paneCmd})
	}

	result.Sessions = make([]Session, 0, len(sessionMap))
	for _, s := range sessionMap {
		result.Sessions = append(result.Sessions, *s)
	}

	return result, nil
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

type PaneBaseIndexQuery struct{}

func (q PaneBaseIndexQuery) Args() []string {
	return []string{"show-options", "-gv", "pane-base-index"}
}

func (q PaneBaseIndexQuery) Parse(output string) (int, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return 0, nil
	}
	var idx int
	_, err := fmt.Sscanf(output, "%d", &idx)
	return idx, err
}
