package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/MSmaili/tms/internal/manifest"
	"github.com/MSmaili/tms/internal/tmux"
	"github.com/spf13/cobra"
)

var saveCmd = &cobra.Command{
	Use:   "save [.]",
	Short: "Save current tmux session to workspace",
	Long: `Save the current tmux session state to a workspace configuration file.

By default, saves the current session. Use --all to save all sessions.
Use -n to specify a workspace name or -p for an explicit path.`,
	RunE: runSave,
}

var (
	savePath string
	saveName string
	saveAll  bool
)

func init() {
	rootCmd.AddCommand(saveCmd)
	saveCmd.Flags().StringVarP(&savePath, "path", "p", "", "Path to save workspace file")
	saveCmd.Flags().StringVarP(&saveName, "name", "n", "", "Name for the workspace")
	saveCmd.Flags().BoolVar(&saveAll, "all", false, "Save all tmux sessions")

	saveCmd.ValidArgs = []string{"."}
	saveCmd.RegisterFlagCompletionFunc("name", completeWorkspaceNames)
}

func runSave(cmd *cobra.Command, args []string) error {
	if savePath != "" && saveName != "" {
		return fmt.Errorf("cannot use both -p and -n flags")
	}

	client, err := tmux.New()
	if err != nil {
		return fmt.Errorf("initializing tmux client: %w", err)
	}

	result, err := tmux.RunQuery(client, tmux.LoadStateQuery{})
	if err != nil {
		return fmt.Errorf("querying tmux state: %w", err)
	}

	if len(result.Sessions) == 0 {
		return fmt.Errorf("no tmux sessions found")
	}

	var targetSessions []tmux.Session
	var workspacePath string

	if saveAll {
		targetSessions = result.Sessions
	} else {
		if result.CurrentSession == "" {
			return fmt.Errorf("not in a tmux session")
		}

		var found bool
		for _, s := range result.Sessions {
			if s.Name == result.CurrentSession {
				targetSessions = []tmux.Session{s}
				workspacePath = s.WorkspacePath
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("session %q not found", result.CurrentSession)
		}
	}

	outputPath, err := determinePath(args, workspacePath, saveAll)
	if err != nil {
		return err
	}

	workspace := convertToWorkspace(targetSessions)

	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("resolving absolute path: %w", err)
	}

	if err := manifest.Write(workspace, absPath); err != nil {
		return fmt.Errorf("writing workspace: %w", err)
	}

	sessionNames := make([]string, 0, len(targetSessions))
	for _, s := range targetSessions {
		sessionNames = append(sessionNames, s.Name)
	}

	if err := client.ExecuteBatch(buildSetEnvActions(sessionNames, absPath)); err != nil {
		return fmt.Errorf("updating environment: %w", err)
	}

	fmt.Printf("Saved to %s\n", absPath)
	return nil
}

func determinePath(args []string, workspacePath string, requireExplicit bool) (string, error) {
	useDot := len(args) > 0 && args[0] == "."

	if savePath != "" {
		return savePath, nil
	}

	if saveName != "" {
		configDir, err := manifest.GetConfigDir()
		if err != nil {
			return "", fmt.Errorf("getting config dir: %w", err)
		}
		return filepath.Join(configDir, "workspaces", saveName+".yaml"), nil
	}

	if useDot {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("getting current directory: %w", err)
		}
		return filepath.Join(cwd, ".tms.yaml"), nil
	}

	if requireExplicit {
		return "", fmt.Errorf("--all requires -p <path>, -n <name>, or . ")
	}

	if workspacePath != "" {
		return workspacePath, nil
	}

	return "", fmt.Errorf("no workspace path found. Use -p <path>, -n <name>, or . for current directory")
}

func convertToWorkspace(sessions []tmux.Session) *manifest.Workspace {
	ws := &manifest.Workspace{
		Sessions: make(map[string]manifest.WindowList, len(sessions)),
	}

	for _, sess := range sessions {
		windows := make([]manifest.Window, 0, len(sess.Windows))
		for _, w := range sess.Windows {
			win := manifest.Window{
				Name: w.Name,
				Path: w.Path,
			}
			if len(w.Panes) > 1 {
				win.Panes = make([]manifest.Pane, len(w.Panes))
				for j, p := range w.Panes {
					win.Panes[j] = manifest.Pane{Path: p.Path}
				}
			}
			windows = append(windows, win)
		}
		ws.Sessions[sess.Name] = windows
	}

	return ws
}
