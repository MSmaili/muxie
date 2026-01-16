package cmd

import (
	"fmt"
	"os"

	"github.com/MSmaili/tmx/internal/domain"
	"github.com/MSmaili/tmx/internal/manifest"
	"github.com/MSmaili/tmx/internal/tmux"
)

func Start(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: tmx start <config-path>")
		os.Exit(1)
	}

	configPath := args[0]

	c := manifest.NewFileLoader(configPath)

	sessions, err := c.Load()
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	tmx, err := tmux.New()

	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	basePaneIndex, err := tmx.BasePaneIndex()
	if err != nil {
		fmt.Println("Error getting pane base index:", err)
		os.Exit(1)
	}

	f, err := createSessions(sessions, tmx, basePaneIndex)

	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	err = tmx.Attach(f)

	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func createSessions(c *manifest.Config, tmx *tmux.TmuxClient, basePaneIndex int) (string, error) {
	var defaultSession string
	for session, windows := range c.Sessions {
		if len(defaultSession) == 0 {
			defaultSession = session
		}

		var window domain.Window
		if len(windows) > 0 {
			window = windows[0]
		}

		windowForCreation := window
		windowForCreation.Command = ""

		err := tmx.CreateSession(session, &windowForCreation)
		if err != nil {
			return "", err
		}

		if len(window.Panes) > 0 {
			if err := createPanes(tmx, session, window.Name, windows[0], basePaneIndex); err != nil {
				return "", fmt.Errorf("create panes for window %s: %w", window.Name, err)
			}
		} else if window.Command != "" {
			// Single pane with command
			if err := tmx.SendKeys(session, window.Name, basePaneIndex, window.Command); err != nil {
				return "", fmt.Errorf("send keys to window %s: %w", window.Name, err)
			}
		}

		for i := 1; i < len(windows); i++ {
			wo := windows[i]

			windowForCreation := wo
			windowForCreation.Command = ""

			err := tmx.CreateWindow(session, wo.Name, windowForCreation)
			if err != nil {
				return "", err
			}

			if len(wo.Panes) > 0 {
				if err := createPanes(tmx, session, wo.Name, windows[i], basePaneIndex); err != nil {
					return "", fmt.Errorf("create panes for window %s: %w", wo.Name, err)
				}
			} else if wo.Command != "" {
				if err := tmx.SendKeys(session, wo.Name, basePaneIndex, wo.Command); err != nil {
					return "", fmt.Errorf("send keys to window %s: %w", wo.Name, err)
				}
			}
		}
	}

	return defaultSession, nil
}

func createPanes(tmx *tmux.TmuxClient, session, window string, w domain.Window, basePaneIndex int) error {
	if len(w.Panes) == 0 {
		return nil
	}

	if w.Panes[0].Command != "" {
		if err := tmx.SendKeys(session, window, basePaneIndex, w.Panes[0].Command); err != nil {
			return fmt.Errorf("send keys to pane %d: %w", basePaneIndex, err)
		}
	}

	for i := 1; i < len(w.Panes); i++ {
		pane := w.Panes[i]
		if err := tmx.SplitPane(session, window, pane); err != nil {
			return fmt.Errorf("split pane %d: %w", i, err)
		}

		if pane.Command != "" {
			paneIndex := basePaneIndex + i
			if err := tmx.SendKeys(session, window, paneIndex, pane.Command); err != nil {
				return fmt.Errorf("send keys to pane %d: %w", paneIndex, err)
			}
		}
	}

	return nil
}
