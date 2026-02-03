package manifest

import (
	"fmt"
	"strings"
)

type ValidationError struct {
	Field   string
	Message string
}

func ToError(errs []ValidationError) error {
	if len(errs) == 0 {
		return nil
	}
	messages := make([]string, len(errs))
	for i, e := range errs {
		if e.Field != "" {
			messages[i] = fmt.Sprintf("%s: %s", e.Field, e.Message)
		} else {
			messages[i] = e.Message
		}
	}
	return fmt.Errorf("workspace validation failed:\n  - %s", strings.Join(messages, "\n  - "))
}

func Validate(ws *Workspace) []ValidationError {
	if ws == nil {
		return []ValidationError{{Message: "workspace is nil"}}
	}

	if len(ws.Sessions) == 0 {
		return []ValidationError{{Message: "workspace has no sessions defined\nHint: Add at least one session to your workspace file"}}
	}

	errs := make([]ValidationError, 0, len(ws.Sessions))
	seenSessions := make(map[string]bool, len(ws.Sessions))

	for _, sess := range ws.Sessions {
		errs = validateSession(sess, seenSessions, errs)
	}

	return errs
}

func validateSession(sess Session, seen map[string]bool, errs []ValidationError) []ValidationError {
	if strings.TrimSpace(sess.Name) == "" {
		return append(errs, ValidationError{Message: "session name cannot be empty"})
	}

	if seen[sess.Name] {
		return append(errs, ValidationError{
			Field:   fmt.Sprintf("session.%s", sess.Name),
			Message: "duplicate session name",
		})
	}
	seen[sess.Name] = true

	if len(sess.Windows) == 0 {
		return append(errs, ValidationError{
			Field:   fmt.Sprintf("session.%s", sess.Name),
			Message: "has no windows defined",
		})
	}

	return validateWindows(sess.Name, sess.Windows, errs)
}

func validateWindows(sessionName string, windows []Window, errs []ValidationError) []ValidationError {
	seenWindows := make(map[string]bool, len(windows))

	for i, window := range windows {
		windowName := window.Name
		if windowName == "" {
			windowName = fmt.Sprintf("window-%d", i)
		}

		if seenWindows[windowName] {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("session.%s.window.%s", sessionName, windowName),
				Message: "duplicate window name",
			})
			continue
		}
		seenWindows[windowName] = true

		if err := validateZoomedPanes(sessionName, windowName, window.Panes); err != nil {
			errs = append(errs, *err)
		}
	}

	return errs
}

func validateZoomedPanes(sessionName, windowName string, panes []Pane) *ValidationError {
	zoomedCount := 0
	for _, pane := range panes {
		if pane.Zoom {
			zoomedCount++
			if zoomedCount > 1 {
				return &ValidationError{
					Field:   fmt.Sprintf("session.%s.window.%s", sessionName, windowName),
					Message: fmt.Sprintf("has %d panes with zoom=true (only one allowed per window)", zoomedCount),
				}
			}
		}
	}
	return nil
}
