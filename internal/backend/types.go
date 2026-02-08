package backend

type StateResult struct {
	Sessions []Session
	Active   ActiveContext
}

type Session struct {
	Name    string
	Windows []Window
}

type Window struct {
	Name   string
	Path   string
	Layout string
	Panes  []Pane
}

type Pane struct {
	Index   int
	Path    string
	Command string
}

type ActiveContext struct {
	Session string
	Window  string
	Pane    int
	Path    string
}
