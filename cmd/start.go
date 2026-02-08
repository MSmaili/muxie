package cmd

import (
	"fmt"

	"github.com/MSmaili/muxie/internal/backend"
	"github.com/MSmaili/muxie/internal/converter"
	"github.com/MSmaili/muxie/internal/logger"
	"github.com/MSmaili/muxie/internal/manifest"
	"github.com/MSmaili/muxie/internal/plan"
	"github.com/MSmaili/muxie/internal/state"
	"github.com/spf13/cobra"
)

var (
	dryRun bool
	force  bool
)

var startCmd = &cobra.Command{
	Use:   "start [workspace-name-or-path]",
	Short: "Start a tmux workspace",
	RunE:  runStart,
}

func init() {
	startCmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Print plan without executing")
	startCmd.Flags().BoolVarP(&force, "force", "f", false, "Kill extra sessions/windows and recreate mismatched")
	rootCmd.AddCommand(startCmd)

	startCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeWorkspaceNames(cmd, args, toComplete)
	}
}

func runStart(cmd *cobra.Command, args []string) error {
	workspace, _, err := loadWorkspaceFromArgs(args)
	if err != nil {
		return err
	}

	b, err := backend.Detect()
	if err != nil {
		return fmt.Errorf("failed to detect backend: %w", err)
	}

	p, err := buildPlan(b, workspace)
	if err != nil {
		return err
	}

	return executePlan(b, p, workspace)
}

func loadWorkspaceFromArgs(args []string) (*manifest.Workspace, string, error) {
	var nameOrPath string
	if len(args) > 0 {
		nameOrPath = args[0]
	}

	resolver := manifest.NewResolver()
	workspacePath, err := resolver.Resolve(nameOrPath)
	if err != nil {
		return nil, "", err
	}

	loader := manifest.NewFileLoader(workspacePath)
	workspace, err := loader.Load()
	if err != nil {
		return nil, "", fmt.Errorf("loading workspace: %w", err)
	}

	if errs := manifest.Validate(workspace); len(errs) > 0 {
		return nil, "", manifest.ToError(errs)
	}

	return workspace, workspacePath, nil
}

func buildPlan(b backend.Backend, workspace *manifest.Workspace) (*plan.Plan, error) {
	desired := converter.ManifestToState(workspace)

	result, err := b.QueryState()
	if err != nil {
		result = backend.StateResult{}
	}
	actual := converter.BackendResultToState(result)

	diff := state.Compare(desired, actual)
	planDiff := converter.StateDiffToPlanDiff(diff, desired)

	strategy := selectStrategy()
	return strategy.Plan(planDiff), nil
}

func selectStrategy() plan.Strategy {
	if force {
		return &plan.ForceStrategy{}
	}
	return &plan.MergeStrategy{}
}

func executePlan(b backend.Backend, p *plan.Plan, workspace *manifest.Workspace) error {
	if p.IsEmpty() {
		logger.Info("Workspace already up to date")
		return attachToSession(b, workspace)
	}

	if dryRun {
		printDryRun(b, p)
		return nil
	}

	if err := b.Apply(toBackendActions(p.Actions)); err != nil {
		return fmt.Errorf("failed to execute plan: %w\nHint: Check tmux server logs or try with --dry-run to see planned actions", err)
	}

	return attachToSession(b, workspace)
}

func printDryRun(b backend.Backend, p *plan.Plan) {
	logger.Info("Dry run - actions to execute:")
	for _, line := range b.DryRun(toBackendActions(p.Actions)) {
		logger.Plain("  %s", line)
	}
}

func toBackendActions(actions []plan.Action) []backend.Action {
	result := make([]backend.Action, len(actions))
	for i, a := range actions {
		result[i] = a
	}
	return result
}

func attachToSession(b backend.Backend, workspace *manifest.Workspace) error {
	if len(workspace.Sessions) > 0 {
		return b.Attach(workspace.Sessions[0].Name)
	}
	return nil
}
