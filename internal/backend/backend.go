package backend

import "context"

type Action interface {
	Comment() string
	Validate() error
}

type Backend interface {
	Name() string
	QueryState(context.Context) (StateResult, error)
	Apply(context.Context, []Action) error
	DryRun([]Action) ([]string, error)
	Attach(context.Context, string) error
	Switch(context.Context, string) error
}
