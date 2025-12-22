package tmux

import "github.com/MSmaili/tms/internal/domain"

type Client interface {
	ListSessions() (map[string][]domain.Window, error)
	ListWindows(session string) ([]domain.Window, error)
	HasSession(name string) bool

	CreateSession(name string, opts *domain.Window) error
	CreateWindow(session string, name string, opts *domain.Window) error

	SetLayout(session string, window string, layout string) error

	Attach(session string) error

	KillSession(name string) error
	KillWindow(session, window string) error
}
