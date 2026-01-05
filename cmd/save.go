package cmd

import (
	"fmt"
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
	if err := validateSaveFlags(); err != nil {
		return err
	}

	client, err := tmux.New()
	if err != nil {
		return fmt.Errorf("initializing tmux client: %w", err)
	}

	sessions, existingPath, err := getTargetSessions(client)
	if err != nil {
		return err
	}

	outputPath, err := determineSavePath(args, existingPath)
	if err != nil {
		return err
	}

	return saveWorkspace(client, sessions, outputPath)
}

func validateSaveFlags() error {
	if savePath != "" && saveName != "" {
		return fmt.Errorf("cannot use both -p and -n flags")
	}
	return nil
}

func getTargetSessions(client tmux.Client) ([]tmux.Session, string, error) {
	result, err := tmux.RunQuery(client, tmux.LoadStateQuery{})
	if err != nil {
		return nil, "", fmt.Errorf("querying tmux state: %w", err)
	}

	if len(result.Sessions) == 0 {
		return nil, "", fmt.Errorf("no tmux sessions found")
	}

	if saveAll {
		return result.Sessions, "", nil
	}

	return findCurrentSession(result)
}

func findCurrentSession(result tmux.LoadStateResult) ([]tmux.Session, string, error) {
	if result.CurrentSession == "" {
		return nil, "", fmt.Errorf("not in a tmux session")
	}

	for _, s := range result.Sessions {
		if s.Name == result.CurrentSession {
			return []tmux.Session{s}, s.WorkspacePath, nil
		}
	}

	return nil, "", fmt.Errorf("session %q not found", result.CurrentSession)
}

func determineSavePath(args []string, existingPath string) (string, error) {
	if savePath != "" {
		return savePath, nil
	}

	resolver := manifest.NewResolver()

	if saveName != "" {
		return resolver.NamedPath(saveName)
	}

	if len(args) > 0 && args[0] == "." {
		return resolver.LocalPath()
	}

	if saveAll {
		return "", fmt.Errorf("--all requires -p <path>, -n <name>, or .")
	}

	if existingPath != "" {
		return existingPath, nil
	}

	return "", fmt.Errorf("no workspace path found. Use -p <path>, -n <name>, or . for current directory")
}

func saveWorkspace(client tmux.Client, sessions []tmux.Session, outputPath string) error {
	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("resolving absolute path: %w", err)
	}

	workspace := convertToWorkspace(sessions)

	if err := manifest.Write(workspace, absPath); err != nil {
		return fmt.Errorf("writing workspace: %w", err)
	}

	if err := updateSessionEnv(client, sessions, absPath); err != nil {
		return err
	}

	fmt.Printf("Saved to %s\n", absPath)
	return nil
}

func updateSessionEnv(client tmux.Client, sessions []tmux.Session, path string) error {
	names := make([]string, len(sessions))
	for i, s := range sessions {
		names[i] = s.Name
	}

	if err := client.ExecuteBatch(buildSetEnvActions(names, path)); err != nil {
		return fmt.Errorf("updating environment: %w", err)
	}
	return nil
}

func convertToWorkspace(sessions []tmux.Session) *manifest.Workspace {
	ws := &manifest.Workspace{
		Sessions: make(map[string]manifest.WindowList, len(sessions)),
	}

	for _, sess := range sessions {
		ws.Sessions[sess.Name] = convertWindows(sess.Windows)
	}

	return ws
}

func convertWindows(windows []tmux.Window) []manifest.Window {
	result := make([]manifest.Window, len(windows))
	for i, w := range windows {
		result[i] = manifest.Window{
			Name: w.Name,
			Path: w.Path,
		}
		if len(w.Panes) > 1 {
			result[i].Panes = convertPanes(w.Panes)
		}
	}
	return result
}

func convertPanes(panes []tmux.Pane) []manifest.Pane {
	result := make([]manifest.Pane, len(panes))
	for i, p := range panes {
		result[i] = manifest.Pane{Path: p.Path}
	}
	return result
}
