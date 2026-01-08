package plan

import "fmt"

type Strategy interface {
	Plan(diff Diff) *Plan
}

type MergeStrategy struct {
	PaneBaseIndex int
}

func (s *MergeStrategy) Plan(diff Diff) *Plan {
	plan := &Plan{Actions: []Action{}}
	createMissing(plan, diff, s.PaneBaseIndex)
	return plan
}

type ForceStrategy struct {
	PaneBaseIndex int
}

func (s *ForceStrategy) Plan(diff Diff) *Plan {
	plan := &Plan{Actions: []Action{}}
	killExtra(plan, diff)
	recreateMismatched(plan, diff, s.PaneBaseIndex)
	createMissing(plan, diff, s.PaneBaseIndex)
	return plan
}

func killExtra(plan *Plan, diff Diff) {
	for _, session := range diff.Sessions.Extra {
		plan.Actions = append(plan.Actions, KillSessionAction{Name: session.Name})
	}

	for sessionName, windowDiff := range diff.Windows {
		for _, window := range windowDiff.Extra {
			plan.Actions = append(plan.Actions, KillWindowAction{
				Target: fmt.Sprintf("%s:%s", sessionName, window.Name),
			})
		}
	}
}

func recreateMismatched(plan *Plan, diff Diff, paneBaseIndex int) {
	for sessionName, windowDiff := range diff.Windows {
		for _, mismatch := range windowDiff.Mismatched {
			plan.Actions = append(plan.Actions, KillWindowAction{
				Target: fmt.Sprintf("%s:%s", sessionName, mismatch.Actual.Name),
			})
			createWindow(plan, sessionName, mismatch.Desired, paneBaseIndex)
		}
	}
}

func createMissing(plan *Plan, diff Diff, paneBaseIndex int) {
	for _, session := range diff.Sessions.Missing {
		createSession(plan, session, paneBaseIndex)
	}

	for sessionName, windowDiff := range diff.Windows {
		for _, window := range windowDiff.Missing {
			createWindow(plan, sessionName, window, paneBaseIndex)
		}
	}
}

func createSession(plan *Plan, session Session, paneBaseIndex int) {
	if len(session.Windows) == 0 {
		return
	}

	firstWindow := session.Windows[0]
	plan.Actions = append(plan.Actions, CreateSessionAction{
		Name:       session.Name,
		WindowName: firstWindow.Name,
		Path:       firstWindow.Path,
	})
	addPanesAndCommands(plan, session.Name, firstWindow, paneBaseIndex)

	for _, window := range session.Windows[1:] {
		createWindow(plan, session.Name, window, paneBaseIndex)
	}
}

func createWindow(plan *Plan, sessionName string, window Window, paneBaseIndex int) {
	plan.Actions = append(plan.Actions, CreateWindowAction{
		Session: sessionName,
		Name:    window.Name,
		Path:    window.Path,
	})
	addPanesAndCommands(plan, sessionName, window, paneBaseIndex)
}

func addPanesAndCommands(plan *Plan, sessionName string, window Window, paneBaseIndex int) {
	target := fmt.Sprintf("%s:%s", sessionName, window.Name)

	if len(window.Panes) > 1 {
		for _, pane := range window.Panes[1:] {
			plan.Actions = append(plan.Actions, SplitPaneAction{
				Target: target,
				Path:   pane.Path,
			})
		}
	}

	if window.Layout != "" {
		plan.Actions = append(plan.Actions, SelectLayoutAction{
			Target: target,
			Layout: window.Layout,
		})
	}

	for i, pane := range window.Panes {
		if pane.Command != "" {
			plan.Actions = append(plan.Actions, SendKeysAction{
				Target:  fmt.Sprintf("%s.%d", target, i+paneBaseIndex),
				Command: pane.Command,
			})
		}
		if pane.Zoom {
			plan.Actions = append(plan.Actions, ZoomPaneAction{
				Target: fmt.Sprintf("%s.%d", target, i+paneBaseIndex),
			})
		}
	}
}
