package backend

type Action interface {
	Comment() string
	Validate() error
}

type Backend interface {
	Name() string
	QueryState() (StateResult, error)
	Apply(actions []Action) error
	DryRun(actions []Action) []string
	Attach(session string) error
	Switch(target string) error
}
