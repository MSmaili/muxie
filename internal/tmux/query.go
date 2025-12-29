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

type LoadStateQuery struct{}

func (q LoadStateQuery) Args() []string {
	return []string{"list-panes", "-a", "-F", "#{session_name}|#{window_name}|#{pane_current_path}|#{pane_current_command}|#{TMS_WORKSPACE_PATH}"}
}

func (q LoadStateQuery) Parse(output string) ([]Session, error) {
	if output == "" {
		return []Session{}, nil
	}

	sessionMap := make(map[string]*Session)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 5 {
			continue
		}

		sessName, winName, panePath, paneCmd, workspacePath := parts[0], parts[1], parts[2], parts[3], parts[4]

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

	sessions := make([]Session, 0, len(sessionMap))
	for _, s := range sessionMap {
		sessions = append(sessions, *s)
	}

	return sessions, nil
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

// GetCurrentSession returns the current tmux session name from $TMUX env var
// Returns empty string if not in a tmux session
func GetCurrentSession() string {
	tmuxEnv := strings.TrimSpace(os.Getenv("TMUX"))
	if tmuxEnv == "" {
		return ""
	}
	parts := strings.Split(tmuxEnv, ",")
	if len(parts) < 3 {
		return ""
	}
	return parts[2]
}
