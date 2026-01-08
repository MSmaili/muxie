package manifest

import (
	"strings"
	"testing"
)

func TestValidate_ValidWorkspace(t *testing.T) {
	ws := &Workspace{
		Sessions: map[string]WindowList{
			"dev": {
				{Name: "editor", Path: "/home/user"},
				{Name: "terminal", Path: "/home/user"},
			},
		},
	}

	err := Validate(ws)
	if err != nil {
		t.Errorf("expected valid workspace to pass validation, got: %v", err)
	}
}

func TestValidate_NilWorkspace(t *testing.T) {
	err := Validate(nil)
	if err == nil {
		t.Error("expected error for nil workspace")
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Errorf("expected 'nil' in error, got: %v", err)
	}
}

func TestValidate_EmptySessions(t *testing.T) {
	ws := &Workspace{
		Sessions: map[string]WindowList{},
	}

	err := Validate(ws)
	if err == nil {
		t.Error("expected error for empty sessions")
	}
	if !strings.Contains(err.Error(), "no sessions") {
		t.Errorf("expected 'no sessions' in error, got: %v", err)
	}
}

func TestValidate_EmptySessionName(t *testing.T) {
	ws := &Workspace{
		Sessions: map[string]WindowList{
			"": {
				{Name: "editor"},
			},
		},
	}

	err := Validate(ws)
	if err == nil {
		t.Error("expected error for empty session name")
	}
	if !strings.Contains(err.Error(), "session name cannot be empty") {
		t.Errorf("expected 'session name cannot be empty' in error, got: %v", err)
	}
}

func TestValidate_EmptyWindowList(t *testing.T) {
	ws := &Workspace{
		Sessions: map[string]WindowList{
			"dev": {},
		},
	}

	err := Validate(ws)
	if err == nil {
		t.Error("expected error for empty window list")
	}
	if !strings.Contains(err.Error(), "no windows") {
		t.Errorf("expected 'no windows' in error, got: %v", err)
	}
}

func TestValidate_DuplicateWindowNames(t *testing.T) {
	ws := &Workspace{
		Sessions: map[string]WindowList{
			"dev": {
				{Name: "editor"},
				{Name: "editor"},
			},
		},
	}

	err := Validate(ws)
	if err == nil {
		t.Error("expected error for duplicate window names")
	}
	if !strings.Contains(err.Error(), "duplicate window name") {
		t.Errorf("expected 'duplicate window name' in error, got: %v", err)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	ws := &Workspace{
		Sessions: map[string]WindowList{
			"dev": {},
			"":    {{Name: "test"}},
		},
	}

	err := Validate(ws)
	if err == nil {
		t.Error("expected error for multiple validation issues")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "validation failed") {
		t.Errorf("expected 'validation failed' in error, got: %v", err)
	}
}

func TestValidate_WindowsWithoutNames(t *testing.T) {
	ws := &Workspace{
		Sessions: map[string]WindowList{
			"dev": {
				{Path: "/home/user"},
				{Path: "/home/user"},
			},
		},
	}

	// Should pass - windows without names get auto-generated names (window-0, window-1)
	err := Validate(ws)
	if err != nil {
		t.Errorf("expected windows without names to be valid, got: %v", err)
	}
}

func TestValidate_MultipleZoomedPanes(t *testing.T) {
	ws := &Workspace{
		Sessions: map[string]WindowList{
			"dev": {
				{
					Name: "editor",
					Panes: []Pane{
						{Path: "/home/user", Zoom: true},
						{Path: "/home/user", Zoom: true},
					},
				},
			},
		},
	}

	err := Validate(ws)
	if err == nil {
		t.Error("expected error for multiple zoomed panes in one window")
	}
	if !strings.Contains(err.Error(), "zoom=true") {
		t.Errorf("expected 'zoom=true' in error, got: %v", err)
	}
}

func TestValidate_SingleZoomedPane(t *testing.T) {
	ws := &Workspace{
		Sessions: map[string]WindowList{
			"dev": {
				{
					Name: "editor",
					Panes: []Pane{
						{Path: "/home/user", Zoom: true},
						{Path: "/home/user"},
					},
				},
			},
		},
	}

	err := Validate(ws)
	if err != nil {
		t.Errorf("expected single zoomed pane to be valid, got: %v", err)
	}
}
