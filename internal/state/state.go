package state

type State struct {
	Sessions map[string]*Session
}

type Session struct {
	ID      string
	Name    string
	Windows []*Window
}

type Window struct {
	ID     string
	Name   string
	Path   string
	Layout string
	Panes  []*Pane
}

type Pane struct {
	ID      string
	Index   int
	Path    string
	Command string
	Zoom    bool
}

func NewState() *State {
	return &State{Sessions: make(map[string]*Session)}
}

func (s *State) AddSession(name string) *Session {
	session := &Session{Name: name}
	s.Sessions[name] = session
	return session
}
