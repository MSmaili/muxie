package tmux

type Window struct {
	ID     string
	Name   string
	Index  int
	Path   string
	Layout string
	Panes  []Pane
}

type Pane struct {
	ID      string
	Index   int
	Path    string
	Command string
	Zoom    bool
}
