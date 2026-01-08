package plan

import (
	"errors"
	"fmt"
)

type Action interface {
	ToCommand() string
	Comment() string
	Validate() error
}

type CreateSessionAction struct {
	Name       string
	WindowName string
	Path       string
}

func (a CreateSessionAction) ToCommand() string {
	cmd := fmt.Sprintf("new-session -d -s %q", a.Name)
	if a.WindowName != "" {
		cmd += fmt.Sprintf(" -n %q", a.WindowName)
	}
	if a.Path != "" {
		cmd += fmt.Sprintf(" -c %q", a.Path)
	}
	return cmd
}

func (a CreateSessionAction) Comment() string {
	return fmt.Sprintf("# Create session: %s", a.Name)
}

func (a CreateSessionAction) Validate() error {
	if a.Name == "" {
		return errors.New("session name cannot be empty")
	}
	return nil
}

type CreateWindowAction struct {
	Session string
	Name    string
	Path    string
}

func (a CreateWindowAction) ToCommand() string {
	return fmt.Sprintf("new-window -t %q -n %q -c %q", a.Session, a.Name, a.Path)
}

func (a CreateWindowAction) Comment() string {
	return fmt.Sprintf("# Create window: %s:%s", a.Session, a.Name)
}

func (a CreateWindowAction) Validate() error {
	if a.Session == "" || a.Name == "" {
		return errors.New("window session and name cannot be empty")
	}
	return nil
}

type SplitPaneAction struct {
	Target string
	Path   string
}

func (a SplitPaneAction) ToCommand() string {
	return fmt.Sprintf("split-window -t %q -c %q", a.Target, a.Path)
}

func (a SplitPaneAction) Comment() string {
	return fmt.Sprintf("# Split pane in: %s", a.Target)
}

func (a SplitPaneAction) Validate() error {
	if a.Target == "" {
		return errors.New("split pane target cannot be empty")
	}
	return nil
}

type SendKeysAction struct {
	Target  string
	Command string
}

func (a SendKeysAction) ToCommand() string {
	return fmt.Sprintf("send-keys -t %q %q Enter", a.Target, a.Command)
}

func (a SendKeysAction) Comment() string {
	return fmt.Sprintf("# Send command to: %s", a.Target)
}

func (a SendKeysAction) Validate() error {
	if a.Target == "" {
		return errors.New("send keys target cannot be empty")
	}
	return nil
}

type KillSessionAction struct {
	Name string
}

func (a KillSessionAction) ToCommand() string {
	return fmt.Sprintf("kill-session -t %q", a.Name)
}

func (a KillSessionAction) Comment() string {
	return fmt.Sprintf("# Kill session: %s", a.Name)
}

func (a KillSessionAction) Validate() error {
	if a.Name == "" {
		return errors.New("kill session name cannot be empty")
	}
	return nil
}

type KillWindowAction struct {
	Target string
}

func (a KillWindowAction) ToCommand() string {
	return fmt.Sprintf("kill-window -t %q", a.Target)
}

func (a KillWindowAction) Comment() string {
	return fmt.Sprintf("# Kill window: %s", a.Target)
}

func (a KillWindowAction) Validate() error {
	if a.Target == "" {
		return errors.New("kill window target cannot be empty")
	}
	return nil
}


type SelectLayoutAction struct {
	Target string
	Layout string
}

func (a SelectLayoutAction) ToCommand() string {
	return fmt.Sprintf("select-layout -t %q %q", a.Target, a.Layout)
}

func (a SelectLayoutAction) Comment() string {
	return fmt.Sprintf("# Set layout: %s -> %s", a.Target, a.Layout)
}

func (a SelectLayoutAction) Validate() error {
	if a.Target == "" || a.Layout == "" {
		return errors.New("select layout target and layout cannot be empty")
	}
	return nil
}

type ZoomPaneAction struct {
	Target string
}

func (a ZoomPaneAction) ToCommand() string {
	return fmt.Sprintf("resize-pane -Z -t %q", a.Target)
}

func (a ZoomPaneAction) Comment() string {
	return fmt.Sprintf("# Zoom pane: %s", a.Target)
}

func (a ZoomPaneAction) Validate() error {
	if a.Target == "" {
		return errors.New("zoom pane target cannot be empty")
	}
	return nil
}
