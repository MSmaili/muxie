package converter

import (
	"github.com/MSmaili/muxie/internal/backend"
	"github.com/MSmaili/muxie/internal/state"
)

func BackendResultToState(result backend.StateResult) *state.State {
	s := state.NewState()
	for _, sess := range result.Sessions {
		session := s.AddSession(sess.Name)
		for _, w := range sess.Windows {
			session.Windows = append(session.Windows, backendWindowToState(w))
		}
	}
	return s
}

func backendWindowToState(w backend.Window) *state.Window {
	window := &state.Window{Name: w.Name, Path: w.Path, Layout: w.Layout}
	for _, p := range w.Panes {
		window.Panes = append(window.Panes, &state.Pane{Path: p.Path, Command: p.Command})
	}
	return window
}
