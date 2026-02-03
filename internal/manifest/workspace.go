package manifest

type Workspace struct {
	Sessions []Session `json:"sessions" yaml:"sessions"`
}

type Session struct {
	Name    string   `json:"name" yaml:"name"`
	Root    string   `json:"root,omitempty" yaml:"root,omitempty"`
	Windows []Window `json:"windows" yaml:"windows"`
}

type Window struct {
	Name    string `json:"name,omitempty" yaml:"name,omitempty"`
	Path    string `json:"path,omitempty" yaml:"path,omitempty"`
	Index   *int   `json:"index,omitempty" yaml:"index,omitempty"`
	Layout  string `json:"layout,omitempty" yaml:"layout,omitempty"`
	Command string `json:"command,omitempty" yaml:"command,omitempty"`
	Panes   []Pane `json:"panes,omitempty" yaml:"panes,omitempty"`
}

type Pane struct {
	Path    string `json:"path,omitempty" yaml:"path,omitempty"`
	Command string `json:"command,omitempty" yaml:"command,omitempty"`
	Split   string `json:"split,omitempty" yaml:"split,omitempty"`
	Size    int    `json:"size,omitempty" yaml:"size,omitempty"`
	Zoom    bool   `json:"zoom,omitempty" yaml:"zoom,omitempty"`
}
