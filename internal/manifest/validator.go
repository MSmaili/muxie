package manifest

import (
	"fmt"
	"strings"
)

func Validate(ws *Workspace) error {
	if ws == nil {
		return fmt.Errorf("workspace is nil")
	}

	if len(ws.Sessions) == 0 {
		return fmt.Errorf("workspace has no sessions defined\nHint: Add at least one session to your workspace file")
	}

	var errs []string
	seenSessions := make(map[string]bool)

	for sessionName, windows := range ws.Sessions {
		if strings.TrimSpace(sessionName) == "" {
			errs = append(errs, "session name cannot be empty")
			continue
		}

		if seenSessions[sessionName] {
			errs = append(errs, fmt.Sprintf("duplicate session name: %q", sessionName))
		}
		seenSessions[sessionName] = true

		if len(windows) == 0 {
			errs = append(errs, fmt.Sprintf("session %q has no windows defined", sessionName))
			continue
		}

		seenWindows := make(map[string]bool)
		for i, window := range windows {
			windowName := window.Name
			if windowName == "" {
				windowName = fmt.Sprintf("window-%d", i)
			}

			if seenWindows[windowName] {
				errs = append(errs, fmt.Sprintf("session %q has duplicate window name: %q", sessionName, windowName))
			}
			seenWindows[windowName] = true

			// Check zoom: only one pane per window can be zoomed
			zoomedCount := 0
			for _, pane := range window.Panes {
				if pane.Zoom {
					zoomedCount++
				}
			}
			if zoomedCount > 1 {
				errs = append(errs, fmt.Sprintf("window %q in session %q has %d panes with zoom=true (only one allowed per window)", windowName, sessionName, zoomedCount))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("workspace validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}
