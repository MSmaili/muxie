package cmd

import (
	"errors"
	"fmt"

	applist "github.com/MSmaili/hetki/internal/app/list"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list [workspaces|sessions]",
	Short: "List workspaces or sessions",
	Long: `List workspace files or running tmux sessions.

Examples:
	  hetki list                              # List workspace names
	  hetki list workspaces --sessions        # workspace:session
	  hetki list sessions --windows --format=tree  # Pretty tree view
	  hetki list sessions --windows --format=json  # JSON output`,
	RunE: runList,
}

var (
	listSessions  bool
	listWindows   bool
	listPanes     bool
	listFormat    string
	listDelimiter string
	listCurrent   bool
	listMarker    string
)

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().BoolVarP(&listSessions, "sessions", "s", false, "Include sessions")
	listCmd.Flags().BoolVarP(&listWindows, "windows", "w", false, "Include windows")
	listCmd.Flags().BoolVarP(&listPanes, "panes", "p", false, "Include panes")
	listCmd.Flags().StringVarP(&listFormat, "format", "f", "flat", "Output format: flat, indent, tree, json")
	listCmd.Flags().StringVarP(&listDelimiter, "delimiter", "d", ":", "Delimiter for flat output")
	listCmd.Flags().BoolVarP(&listCurrent, "current", "c", false, "Only show current session")
	listCmd.Flags().StringVarP(&listMarker, "marker", "m", "", "Prefix for current session/window (e.g. '➤ ')")

	listCmd.ValidArgs = []string{"workspaces", "sessions"}
	listCmd.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"flat", "indent", "tree", "json"}, cobra.ShellCompDirectiveNoFileComp
	})
}

func runList(cmd *cobra.Command, args []string) error {
	mode := applist.ModeWorkspaces
	if len(args) > 0 {
		mode = args[0]
	}

	if err := validateListFlags(mode); err != nil {
		return err
	}

	result, listErr := applist.NewService(detectBackend).Run(applist.Options{
		Mode:            mode,
		IncludeSessions: listSessions,
		IncludeWindows:  listWindows,
		IncludePanes:    listPanes,
		CurrentOnly:     listCurrent,
		Marker:          listMarker,
	})

	var outputErr error
	if result.NamesOnly {
		outputErr = outputNames(result.Names)
	} else {
		outputErr = outputItems(result.Items)
	}
	return errors.Join(outputErr, listErr)
}

func validateListFlags(mode string) error {
	validFormats := map[string]bool{"flat": true, "indent": true, "tree": true, "json": true}
	if !validFormats[listFormat] {
		return fmt.Errorf("invalid format %q\nValid formats: flat, indent, tree, json\nExample: hetki list --format=tree", listFormat)
	}
	if mode == applist.ModeWorkspaces {
		if listWindows && !listSessions {
			return fmt.Errorf("--windows requires --sessions\nExample: hetki list workspaces --sessions --windows")
		}
		if listCurrent {
			return fmt.Errorf("--current only works with sessions\nExample: hetki list sessions --current")
		}
		if listMarker != "" {
			return fmt.Errorf("--marker only works with sessions\nExample: hetki list sessions --marker '➤ '")
		}
	}
	if listPanes && !listWindows {
		return fmt.Errorf("--panes requires --windows\nExample: hetki list sessions --windows --panes")
	}
	return nil
}
