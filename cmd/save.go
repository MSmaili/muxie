package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MSmaili/muxie/internal/logger"
	"github.com/MSmaili/muxie/internal/manifest"
	"github.com/MSmaili/muxie/internal/tmux"
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
		return fmt.Errorf("failed to connect to tmux: %w\nHint: Make sure tmux is running", err)
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
		return fmt.Errorf("cannot use both -p and -n flags\nUse either: muxie save -p <path> OR muxie save -n <name>")
	}
	return nil
}

func getTargetSessions(client tmux.Client) ([]tmux.Session, string, error) {
	result, err := tmux.RunQuery(client, tmux.LoadStateQuery{})
	if err != nil {
		return nil, "", fmt.Errorf("failed to query tmux sessions: %w", err)
	}

	if len(result.Sessions) == 0 {
		return nil, "", fmt.Errorf("no tmux sessions found\nHint: Create a session first with 'tmux new -s <name>'")
	}

	if saveAll {
		return result.Sessions, "", nil
	}

	return findCurrentSession(result)
}

func findCurrentSession(result tmux.LoadStateResult) ([]tmux.Session, string, error) {
	if result.Active.Session == "" {
		return nil, "", fmt.Errorf("not in a tmux session\nHint: Run this command from inside tmux, or use --all with -p/-n/.")
	}

	for _, s := range result.Sessions {
		if s.Name == result.Active.Session {
			return []tmux.Session{s}, s.WorkspacePath, nil
		}
	}

	return nil, "", fmt.Errorf("session %q not found", result.Active.Session)
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
		return "", fmt.Errorf("--all requires a destination\nUse: muxie save --all -p <path>, muxie save --all -n <name>, or muxie save --all .")
	}

	if existingPath != "" {
		return existingPath, nil
	}

	return "", fmt.Errorf("no workspace path found\nHint: This session wasn't started by muxie. Use -p <path>, -n <name>, or . to specify where to save")
}

func saveWorkspace(client tmux.Client, sessions []tmux.Session, outputPath string) error {
	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("resolving absolute path: %w", err)
	}

	workspace := convertToWorkspace(sessions)

	if !saveAll {
		loader := manifest.NewFileLoader(absPath)
		if existing, err := loader.Load(); err == nil {
			workspace = mergeWorkspaces(existing, workspace)
		}
	}

	if err := manifest.Write(workspace, absPath); err != nil {
		return fmt.Errorf("writing workspace: %w", err)
	}

	if err := updateSessionEnv(client, sessions, absPath); err != nil {
		return err
	}

	logger.Success("Saved to %s", absPath)
	return nil
}

func mergeWorkspaces(existing, new *manifest.Workspace) *manifest.Workspace {
	seen := make(map[string]int, len(existing.Sessions))
	for i, sess := range existing.Sessions {
		seen[sess.Name] = i
	}

	for _, sess := range new.Sessions {
		if idx, ok := seen[sess.Name]; ok {
			existing.Sessions[idx] = sess
		} else {
			existing.Sessions = append(existing.Sessions, sess)
		}
	}
	return existing
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
		Sessions: make([]manifest.Session, len(sessions)),
	}

	for i, sess := range sessions {
		ws.Sessions[i] = manifest.Session{
			Name:    sess.Name,
			Windows: convertWindows(sess.Windows),
		}
	}

	return ws
}

func convertWindows(windows []tmux.Window) []manifest.Window {
	result := make([]manifest.Window, len(windows))
	for i, w := range windows {
		result[i] = manifest.Window{
			Name: w.Name,
			Path: contractHomePath(w.Path),
		}
		if len(w.Panes) > 1 {
			result[i].Panes = convertPanes(w.Panes)
		}
	}
	return result
}

func contractHomePath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if strings.HasPrefix(path, home+"/") {
		return "~" + strings.TrimPrefix(path, home)
	}
	if path == home {
		return "~"
	}
	return path
}

func convertPanes(panes []tmux.Pane) []manifest.Pane {
	result := make([]manifest.Pane, len(panes))
	for i, p := range panes {
		result[i] = manifest.Pane{Path: contractHomePath(p.Path)}
	}
	return result
}
