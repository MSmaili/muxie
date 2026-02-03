package converter

import (
	"fmt"

	"github.com/MSmaili/muxie/internal/manifest"
	"github.com/MSmaili/muxie/internal/state"
	"github.com/MSmaili/muxie/internal/tmux"
)

func ManifestToState(ws *manifest.Workspace) *state.State {
	s := state.NewState()
	for _, sess := range ws.Sessions {
		session := s.AddSession(sess.Name)
		for i, w := range sess.Windows {
			session.Windows = append(session.Windows, manifestWindowToState(w, i))
		}
	}
	return s
}

func manifestWindowToState(w manifest.Window, index int) *state.Window {
	name := w.Name
	if name == "" {
		name = fmt.Sprintf("window-%d", index)
	}
	window := &state.Window{Name: name, Path: w.Path, Layout: w.Layout}
	for _, p := range w.Panes {
		window.Panes = append(window.Panes, &state.Pane{Path: p.Path, Command: p.Command, Zoom: p.Zoom})
	}
	return window
}

func TmuxWindowToState(w tmux.Window) *state.Window {
	window := &state.Window{Name: w.Name, Path: w.Path, Layout: w.Layout}
	for _, p := range w.Panes {
		window.Panes = append(window.Panes, &state.Pane{Path: p.Path, Command: p.Command})
	}
	return window
}
