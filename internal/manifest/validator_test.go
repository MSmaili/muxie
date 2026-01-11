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

	errs := Validate(ws)
	if len(errs) > 0 {
		t.Errorf("expected valid workspace to pass validation, got: %v", errs)
	}
}

func TestValidate_NilWorkspace(t *testing.T) {
	errs := Validate(nil)
	if len(errs) == 0 {
		t.Error("expected error for nil workspace")
	}
	if !strings.Contains(errs[0].Message, "nil") {
		t.Errorf("expected 'nil' in error, got: %v", errs)
	}
}

func TestValidate_EmptySessions(t *testing.T) {
	ws := &Workspace{
		Sessions: map[string]WindowList{},
	}

	errs := Validate(ws)
	if len(errs) == 0 {
		t.Error("expected error for empty sessions")
	}
	if !strings.Contains(errs[0].Message, "no sessions") {
		t.Errorf("expected 'no sessions' in error, got: %v", errs)
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

	errs := Validate(ws)
	if len(errs) == 0 {
		t.Error("expected error for empty session name")
	}
	if !strings.Contains(errs[0].Message, "session name cannot be empty") {
		t.Errorf("expected 'session name cannot be empty' in error, got: %v", errs)
	}
}

func TestValidate_EmptyWindowList(t *testing.T) {
	ws := &Workspace{
		Sessions: map[string]WindowList{
			"dev": {},
		},
	}

	errs := Validate(ws)
	if len(errs) == 0 {
		t.Error("expected error for empty window list")
	}
	if !strings.Contains(errs[0].Message, "no windows") {
		t.Errorf("expected 'no windows' in error, got: %v", errs)
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

	errs := Validate(ws)
	if len(errs) == 0 {
		t.Error("expected error for duplicate window names")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "duplicate window name") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'duplicate window name' in errors, got: %v", errs)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	ws := &Workspace{
		Sessions: map[string]WindowList{
			"dev": {},
			"":    {{Name: "test"}},
		},
	}

	errs := Validate(ws)
	if len(errs) == 0 {
		t.Error("expected error for multiple validation issues")
	}
	if len(errs) < 2 {
		t.Errorf("expected multiple errors, got: %v", errs)
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

	errs := Validate(ws)
	if len(errs) > 0 {
		t.Errorf("expected windows without names to be valid, got: %v", errs)
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

	errs := Validate(ws)
	if len(errs) == 0 {
		t.Error("expected error for multiple zoomed panes in one window")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "zoom=true") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'zoom=true' in errors, got: %v", errs)
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

	errs := Validate(ws)
	if len(errs) > 0 {
		t.Errorf("expected single zoomed pane to be valid, got: %v", errs)
	}
}
