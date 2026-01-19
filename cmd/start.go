package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/MSmaili/muxie/internal/converter"
	"github.com/MSmaili/muxie/internal/logger"
	"github.com/MSmaili/muxie/internal/manifest"
	"github.com/MSmaili/muxie/internal/plan"
	"github.com/MSmaili/muxie/internal/state"
	"github.com/MSmaili/muxie/internal/tmux"
	"github.com/spf13/cobra"
)

const muxieWorkspacePathEnv = "MUXIE_WORKSPACE_PATH"

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
	workspace, workspacePath, err := loadWorkspaceFromArgs(args)
	if err != nil {
		return err
	}

	client, err := tmux.New()
	if err != nil {
		return fmt.Errorf("failed to initialize tmux client: %w", err)
	}

	p, err := buildPlan(client, workspace)
	if err != nil {
		return err
	}

	return executePlan(client, p, workspace, workspacePath)
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

func buildPlan(client tmux.Client, workspace *manifest.Workspace) (*plan.Plan, error) {
	desired := converter.ManifestToState(workspace)

	actual, err := queryTmuxState(client)
	if err != nil {
		return nil, fmt.Errorf("failed to query tmux state: %w", err)
	}

	diff := state.Compare(desired, actual)
	planDiff := converter.StateDiffToPlanDiff(diff, desired)

	strategy := selectStrategy(actual.PaneBaseIndex)
	return strategy.Plan(planDiff), nil
}

func selectStrategy(paneBaseIndex int) plan.Strategy {
	if force {
		return &plan.ForceStrategy{PaneBaseIndex: paneBaseIndex}
	}
	return &plan.MergeStrategy{PaneBaseIndex: paneBaseIndex}
}

func executePlan(client tmux.Client, p *plan.Plan, workspace *manifest.Workspace, workspacePath string) error {
	if p.IsEmpty() {
		logger.Info("Workspace already up to date")
		return attachToSession(client, workspace)
	}

	if dryRun {
		printDryRun(p)
		return nil
	}

	if err := executeActions(client, p, workspace, workspacePath); err != nil {
		return err
	}

	return attachToSession(client, workspace)
}

func printDryRun(p *plan.Plan) {
	logger.Info("Dry run - actions to execute:")
	tmuxActions := converter.PlanActionsToTmux(p.Actions)
	for _, action := range tmuxActions {
		args := action.Args()
		logger.Plain("  tmux %s", strings.Join(args, " "))
	}
}

func executeActions(client tmux.Client, p *plan.Plan, workspace *manifest.Workspace, workspacePath string) error {
	actions := converter.PlanActionsToTmux(p.Actions)

	absPath, err := filepath.Abs(workspacePath)
	if err != nil {
		return fmt.Errorf("resolving workspace path: %w", err)
	}

	sessionNames := make([]string, 0, len(workspace.Sessions))
	for name := range workspace.Sessions {
		sessionNames = append(sessionNames, name)
	}
	actions = append(actions, buildSetEnvActions(sessionNames, absPath)...)

	if err := client.ExecuteBatch(actions); err != nil {
		return fmt.Errorf("failed to execute plan: %w\nHint: Check tmux server logs or try with --dry-run to see planned actions", err)
	}
	return nil
}

func buildSetEnvActions(sessionNames []string, path string) []tmux.Action {
	actions := make([]tmux.Action, 0, len(sessionNames))
	for _, name := range sessionNames {
		actions = append(actions, tmux.SetEnvironment{
			Session: name,
			Name:    muxieWorkspacePathEnv,
			Value:   path,
		})
	}
	return actions
}

func attachToSession(client tmux.Client, workspace *manifest.Workspace) error {
	for sessionName := range workspace.Sessions {
		return client.Attach(sessionName)
	}
	return nil
}

func queryTmuxState(client tmux.Client) (*state.State, error) {
	result, err := tmux.RunQuery(client, tmux.LoadStateQuery{})
	if err != nil {
		return state.NewState(), nil
	}

	s := state.NewState()
	s.PaneBaseIndex = result.PaneBaseIndex

	for _, sess := range result.Sessions {
		session := s.AddSession(sess.Name)
		for _, w := range sess.Windows {
			session.Windows = append(session.Windows, converter.TmuxWindowToState(w))
		}
	}
	return s, nil
}
