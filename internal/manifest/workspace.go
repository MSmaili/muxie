package manifest

type Workspace struct {
	Sessions []Session `yaml:"sessions"`
}

type Session struct {
	Name    string   `yaml:"name"`
	Root    string   `yaml:"root,omitempty"`
	Windows []Window `yaml:"windows"`
}

type Window struct {
	Name    string `yaml:"name,omitempty"`
	Path    string `yaml:"path,omitempty"`
	Index   *int   `yaml:"index,omitempty"`
	Layout  string `yaml:"layout,omitempty"`
	Command string `yaml:"command,omitempty"`
	Panes   []Pane `yaml:"panes,omitempty"`
}

type Pane struct {
	Path    string  `yaml:"path,omitempty"`
	Command string  `yaml:"command,omitempty"`
	Split   *string `yaml:"split,omitempty"`
	Size    *int    `yaml:"size,omitempty"`
	Zoom    bool    `yaml:"zoom,omitempty"`
}
