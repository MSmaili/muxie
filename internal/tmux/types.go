package tmux

type Window struct {
	Name   string
	Path   string
	Layout string
	Panes  []Pane
}

type Pane struct {
	Path    string
	Command string
}

type WindowOpts struct {
	Name    string
	Path    string
	Command string
}

type PaneOpts struct {
	Path  string
	Split string
	Size  int
}
