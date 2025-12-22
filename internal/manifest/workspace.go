package manifest

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Workspace struct {
	Sessions map[string]WindowList `json:"sessions" yaml:"sessions"`
}

type Window struct {
	Name    string `json:"name" yaml:"name"`
	Path    string `json:"path" yaml:"path"`
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
}

type WindowList []Window

func (w *WindowList) UnmarshalJSON(data []byte) error {
	var objForm []Window
	if err := json.Unmarshal(data, &objForm); err == nil {
		*w = objForm
		return nil
	}

	var paths []string
	if err := json.Unmarshal(data, &paths); err == nil {
		windows := make([]Window, len(paths))
		for i, p := range paths {
			windows[i] = Window{
				Path: p,
				Name: inferNameFromPath(p),
			}
		}
		*w = windows
		return nil
	}

	return fmt.Errorf("invalid window list format: %s", string(data))
}

func inferNameFromPath(p string) string {
	if p == "" {
		return ""
	}
	parts := strings.Split(p, "/")
	return parts[len(parts)-1]
}
