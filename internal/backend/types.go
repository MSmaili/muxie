package backend

type StateResult struct {
	Sessions []Session
	Active   ActiveContext
}

type Session struct {
	ID            string
	Name          string
	WorkspacePath string
	Windows       []Window
}

type Window struct {
	ID     string
	Name   string
	Index  int
	Path   string
	Layout string
	Active bool
	Panes  []Pane
}

type Pane struct {
	ID      string
	Index   int
	Path    string
	Command string
	Zoom    bool
	Active  bool
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
