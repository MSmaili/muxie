package cmd

import (
	appsave "github.com/MSmaili/hetki/internal/app/save"
	"github.com/MSmaili/hetki/internal/logger"
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
	outputPath, err := appsave.NewService(detectBackend).Run(cmd.Context(), appsave.Options{
		Path:  savePath,
		Name:  saveName,
		Local: len(args) > 0 && args[0] == ".",
		All:   saveAll,
	})
	if err != nil {
		return err
	}

	logger.Success("Saved to %s", outputPath)
	return nil
}
