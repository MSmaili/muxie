package domain

type Window struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Index   *int   `json:"index"`
	Layout  string `json:"layout"`
	Command string `json:"command"`
}

//todo: what about panes?
