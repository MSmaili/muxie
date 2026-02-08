package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MSmaili/muxie/internal/backend"
	"github.com/MSmaili/muxie/internal/logger"
	"github.com/MSmaili/muxie/internal/manifest"
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

	b, err := backend.Detect()
	if err != nil {
		return fmt.Errorf("failed to detect backend: %w\nHint: Make sure a supported multiplexer is running", err)
	}

	sessions, err := getTargetSessions(b)
	if err != nil {
		return err
	}

	outputPath, err := determineSavePath(args)
	if err != nil {
		return err
	}

	return saveWorkspace(sessions, outputPath)
}

func validateSaveFlags() error {
	if savePath != "" && saveName != "" {
		return fmt.Errorf("cannot use both -p and -n flags\nUse either: muxie save -p <path> OR muxie save -n <name>")
	}
	return nil
}

func getTargetSessions(b backend.Backend) ([]backend.Session, error) {
	result, err := b.QueryState()
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}

	if len(result.Sessions) == 0 {
		return nil, fmt.Errorf("no sessions found\nHint: Create a session first")
	}

	if saveAll {
		return result.Sessions, nil
	}

	return findCurrentSession(result)
}

func findCurrentSession(result backend.StateResult) ([]backend.Session, error) {
	if result.Active.Session == "" {
		return nil, fmt.Errorf("not in a session\nHint: Run this command from inside a multiplexer session, or use --all with -p/-n/.")
	}

	for _, s := range result.Sessions {
		if s.Name == result.Active.Session {
			return []backend.Session{s}, nil
		}
	}

	return nil, fmt.Errorf("session %q not found", result.Active.Session)
}

func determineSavePath(args []string) (string, error) {
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

	return "", fmt.Errorf("no save target specified\nHint: Use -p <path>, -n <name>, or . to specify where to save")
}

func saveWorkspace(sessions []backend.Session, outputPath string) error {
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

func convertToWorkspace(sessions []backend.Session) *manifest.Workspace {
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

func convertWindows(windows []backend.Window) []manifest.Window {
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

func convertPanes(panes []backend.Pane) []manifest.Pane {
	result := make([]manifest.Pane, len(panes))
	for i, p := range panes {
		result[i] = manifest.Pane{Path: contractHomePath(p.Path)}
	}
	return result
}
