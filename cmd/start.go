package cmd

import (
	"fmt"

	"github.com/MSmaili/tms/internal/domain"
	"github.com/MSmaili/tms/internal/manifest"
	"github.com/MSmaili/tms/internal/tmux"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start [workspace-name-or-path]",
	Short: "Start a tmux workspace",
	Long: `Start a tmux workspace from a configuration file.

You can specify:
- A workspace name (looks in ~/.config/tms/workspaces/)
- A file path (./workspace.yaml or /path/to/workspace.yaml)
- Nothing (looks for .tms.yaml in current directory)`,
	RunE: runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	var nameOrPath string
	if len(args) > 0 {
		nameOrPath = args[0]
	}

	resolver := manifest.NewResolver()
	workspacePath, err := resolver.Resolve(nameOrPath)
	if err != nil {
		if nameOrPath == "" {
			fmt.Println("No workspace specified and no .tms.yaml found in current directory")
			fmt.Println("Usage: tms start [workspace-name-or-path]")
		} else {
			fmt.Println("Error:", err)
		}
		return err
	}

	c := manifest.NewFileLoader(workspacePath)

	sessions, err := c.Load()
	if err != nil {
		return fmt.Errorf("loading workspace: %w", err)
	}

	tmx, err := tmux.New()
	if err != nil {
		return fmt.Errorf("initializing tmux client: %w", err)
	}

	basePaneIndex, err := tmx.BasePaneIndex()
	if err != nil {
		return fmt.Errorf("getting pane base index: %w", err)
	}

	firstSession, err := createSessions(sessions, tmx, basePaneIndex)
	if err != nil {
		return fmt.Errorf("creating sessions: %w", err)
	}

	if err := tmx.Attach(firstSession); err != nil {
		return fmt.Errorf("attaching to session: %w", err)
	}

	return nil
}

func createSessions(workspace *manifest.Workspace, tmx *tmux.TmuxClient, basePaneIndex int) (string, error) {
	var firstSession string

	for sessionName, windows := range workspace.Sessions {
		if firstSession == "" {
			firstSession = sessionName
		}

		if err := createSession(tmx, sessionName, windows, basePaneIndex); err != nil {
			return "", fmt.Errorf("session %s: %w", sessionName, err)
		}
	}

	return firstSession, nil
}

func createSession(tmx *tmux.TmuxClient, sessionName string, windows []domain.Window, basePaneIndex int) error {
	if len(windows) == 0 {
		return fmt.Errorf("no windows defined")
	}

	createFirst := func(w domain.Window) error {
		return tmx.CreateSession(sessionName, &w)
	}
	if err := createWindow(createFirst, tmx, sessionName, windows[0], basePaneIndex); err != nil {
		return fmt.Errorf("window %s: %w", windows[0].Name, err)
	}

	for i := 1; i < len(windows); i++ {
		createAdditional := func(w domain.Window) error {
			return tmx.CreateWindow(sessionName, w.Name, w)
		}
		if err := createWindow(createAdditional, tmx, sessionName, windows[i], basePaneIndex); err != nil {
			return fmt.Errorf("window %s: %w", windows[i].Name, err)
		}
	}

	return nil
}

func createWindow(create func(domain.Window) error, tmx *tmux.TmuxClient, sessionName string, window domain.Window, basePaneIndex int) error {
	windowForCreate := window
	windowForCreate.Command = ""

	if err := create(windowForCreate); err != nil {
		return err
	}

	return setupWindow(tmx, sessionName, window, basePaneIndex)
}

func setupWindow(tmx *tmux.TmuxClient, sessionName string, window domain.Window, basePaneIndex int) error {
	if len(window.Panes) > 0 {
		return setupPanes(tmx, sessionName, window.Name, window.Panes, basePaneIndex)
	}

	if window.Command != "" {
		return tmx.SendKeys(sessionName, window.Name, basePaneIndex, window.Command)
	}

	return nil
}

func setupPanes(tmx *tmux.TmuxClient, sessionName, windowName string, panes []domain.Pane, basePaneIndex int) error {
	if panes[0].Command != "" {
		if err := tmx.SendKeys(sessionName, windowName, basePaneIndex, panes[0].Command); err != nil {
			return fmt.Errorf("pane 0: %w", err)
		}
	}

	for i := 1; i < len(panes); i++ {
		if err := createPane(tmx, sessionName, windowName, panes[i], basePaneIndex+i); err != nil {
			return fmt.Errorf("pane %d: %w", i, err)
		}
	}

	return nil
}

func createPane(tmx *tmux.TmuxClient, sessionName, windowName string, pane domain.Pane, paneIndex int) error {
	if err := tmx.SplitPane(sessionName, windowName, pane); err != nil {
		return err
	}

	if pane.Command != "" {
		return tmx.SendKeys(sessionName, windowName, paneIndex, pane.Command)
	}

	return nil
}
