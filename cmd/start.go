package cmd

import (
	"fmt"

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

	loader := manifest.NewFileLoader(workspacePath)
	workspace, err := loader.Load()
	if err != nil {
		return fmt.Errorf("loading workspace: %w", err)
	}

	client, err := tmux.New()
	if err != nil {
		return fmt.Errorf("initializing tmux client: %w", err)
	}

	basePaneIndex, err := client.BasePaneIndex()
	if err != nil {
		return fmt.Errorf("getting pane base index: %w", err)
	}

	firstSession, err := createSessions(workspace, client, basePaneIndex)
	if err != nil {
		return fmt.Errorf("creating sessions: %w", err)
	}

	if err := client.Attach(firstSession); err != nil {
		return fmt.Errorf("attaching to session: %w", err)
	}

	return nil
}

func createSessions(workspace *manifest.Workspace, client *tmux.TmuxClient, basePaneIndex int) (string, error) {
	var firstSession string

	for sessionName, windows := range workspace.Sessions {
		if firstSession == "" {
			firstSession = sessionName
		}

		if err := createSession(client, sessionName, windows, basePaneIndex); err != nil {
			return "", fmt.Errorf("session %s: %w", sessionName, err)
		}
	}

	return firstSession, nil
}

func createSession(client *tmux.TmuxClient, sessionName string, windows []manifest.Window, basePaneIndex int) error {
	if len(windows) == 0 {
		return fmt.Errorf("no windows defined")
	}

	// Create first window with session
	first := windows[0]
	opts := &tmux.WindowOpts{Name: first.Name, Path: first.Path}
	if err := client.CreateSession(sessionName, opts); err != nil {
		return fmt.Errorf("window %s: %w", first.Name, err)
	}
	if err := setupWindow(client, sessionName, first, basePaneIndex); err != nil {
		return fmt.Errorf("window %s: %w", first.Name, err)
	}

	// Create additional windows
	for i := 1; i < len(windows); i++ {
		w := windows[i]
		opts := tmux.WindowOpts{Name: w.Name, Path: w.Path}
		if err := client.CreateWindow(sessionName, opts); err != nil {
			return fmt.Errorf("window %s: %w", w.Name, err)
		}
		if err := setupWindow(client, sessionName, w, basePaneIndex); err != nil {
			return fmt.Errorf("window %s: %w", w.Name, err)
		}
	}

	return nil
}

func setupWindow(client *tmux.TmuxClient, sessionName string, window manifest.Window, basePaneIndex int) error {
	if len(window.Panes) > 0 {
		return setupPanes(client, sessionName, window.Name, window.Panes, basePaneIndex)
	}

	if window.Command != "" {
		return client.SendKeys(sessionName, window.Name, basePaneIndex, window.Command)
	}

	return nil
}

func setupPanes(client *tmux.TmuxClient, sessionName, windowName string, panes []manifest.Pane, basePaneIndex int) error {
	if panes[0].Command != "" {
		if err := client.SendKeys(sessionName, windowName, basePaneIndex, panes[0].Command); err != nil {
			return fmt.Errorf("pane 0: %w", err)
		}
	}

	for i := 1; i < len(panes); i++ {
		if err := createPane(client, sessionName, windowName, panes[i], basePaneIndex+i); err != nil {
			return fmt.Errorf("pane %d: %w", i, err)
		}
	}

	return nil
}

func createPane(client *tmux.TmuxClient, sessionName, windowName string, pane manifest.Pane, paneIndex int) error {
	opts := tmux.PaneOpts{Path: pane.Path, Split: pane.Split, Size: pane.Size}
	if err := client.SplitPane(sessionName, windowName, opts); err != nil {
		return err
	}

	if pane.Command != "" {
		return client.SendKeys(sessionName, windowName, paneIndex, pane.Command)
	}

	return nil
}
