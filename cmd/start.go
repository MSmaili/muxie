package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/MSmaili/tms/internal/logger"
	"github.com/MSmaili/tms/internal/manifest"
	"github.com/MSmaili/tms/internal/plan"
	"github.com/MSmaili/tms/internal/state"
	"github.com/MSmaili/tms/internal/tmux"
	"github.com/spf13/cobra"
)

const tmsWorkspacePathEnv = "TMS_WORKSPACE_PATH"

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
		return fmt.Errorf("failed to connect to tmux: %w\nHint: Make sure tmux server is running", err)
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

	if err := manifest.Validate(workspace); err != nil {
		return nil, "", err
	}

	return workspace, workspacePath, nil
}

func buildPlan(client tmux.Client, workspace *manifest.Workspace) (*plan.Plan, error) {
	desired := manifestToState(workspace)

	actual, err := queryTmuxState(client)
	if err != nil {
		return nil, fmt.Errorf("failed to query tmux state: %w", err)
	}

	diff := state.Compare(desired, actual)
	planDiff := stateDiffToPlanDiff(diff, desired)

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
	tmuxActions := planActionsToTmuxActions(p.Actions)
	for _, action := range tmuxActions {
		args := action.Args()
		logger.Plain("  tmux %s", strings.Join(args, " "))
	}
}

func executeActions(client tmux.Client, p *plan.Plan, workspace *manifest.Workspace, workspacePath string) error {
	actions := planActionsToTmuxActions(p.Actions)

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
			Name:    tmsWorkspacePathEnv,
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

func manifestToState(ws *manifest.Workspace) *state.State {
	s := state.NewState()
	for sessionName, windows := range ws.Sessions {
		session := s.AddSession(sessionName)
		for i, w := range windows {
			session.Windows = append(session.Windows, manifestWindowToState(w, i))
		}
	}
	return s
}

func manifestWindowToState(w manifest.Window, index int) *state.Window {
	name := w.Name
	if name == "" {
		name = fmt.Sprintf("window-%d", index)
	}
	window := &state.Window{Name: name, Path: w.Path, Layout: w.Layout}
	for _, p := range w.Panes {
		window.Panes = append(window.Panes, &state.Pane{Path: p.Path, Command: p.Command, Zoom: p.Zoom})
	}
	return window
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
			session.Windows = append(session.Windows, tmuxWindowToState(w))
		}
	}
	return s, nil
}

func tmuxWindowToState(w tmux.Window) *state.Window {
	window := &state.Window{Name: w.Name, Path: w.Path, Layout: w.Layout}
	for _, p := range w.Panes {
		window.Panes = append(window.Panes, &state.Pane{Path: p.Path, Command: p.Command})
	}
	return window
}

func stateDiffToPlanDiff(sd state.Diff, desired *state.State) plan.Diff {
	pd := plan.Diff{
		Windows: make(map[string]plan.ItemDiff[plan.Window]),
	}

	pd.Sessions.Missing = convertMissingSessions(sd.Sessions.Missing, desired)
	pd.Sessions.Extra = convertExtraSessions(sd.Sessions.Extra)

	for sessionName, wd := range sd.Windows {
		pd.Windows[sessionName] = convertWindowDiff(wd)
	}

	return pd
}

func convertMissingSessions(names []string, desired *state.State) []plan.Session {
	sessions := make([]plan.Session, 0, len(names))
	for _, name := range names {
		session := desired.Sessions[name]
		ps := plan.Session{Name: name}
		for _, w := range session.Windows {
			ps.Windows = append(ps.Windows, stateWindowToPlan(w))
		}
		sessions = append(sessions, ps)
	}
	return sessions
}

func convertExtraSessions(names []string) []plan.Session {
	sessions := make([]plan.Session, 0, len(names))
	for _, name := range names {
		sessions = append(sessions, plan.Session{Name: name})
	}
	return sessions
}

func convertWindowDiff(wd state.ItemDiff[state.Window]) plan.ItemDiff[plan.Window] {
	pwd := plan.ItemDiff[plan.Window]{}

	for _, w := range wd.Missing {
		pwd.Missing = append(pwd.Missing, stateWindowToPlan(&w))
	}
	for _, w := range wd.Extra {
		pwd.Extra = append(pwd.Extra, plan.Window{Name: w.Name, Path: w.Path})
	}
	for _, m := range wd.Mismatched {
		pwd.Mismatched = append(pwd.Mismatched, plan.Mismatch[plan.Window]{
			Desired: stateWindowToPlan(&m.Desired),
			Actual:  stateWindowToPlan(&m.Actual),
		})
	}
	return pwd
}

func stateWindowToPlan(w *state.Window) plan.Window {
	pw := plan.Window{Name: w.Name, Path: w.Path, Layout: w.Layout}
	for _, p := range w.Panes {
		pw.Panes = append(pw.Panes, plan.Pane{Path: p.Path, Command: p.Command, Zoom: p.Zoom})
	}
	return pw
}

func planActionsToTmuxActions(actions []plan.Action) []tmux.Action {
	result := make([]tmux.Action, 0, len(actions))
	for _, a := range actions {
		if ta := planActionToTmuxAction(a); ta != nil {
			result = append(result, ta)
		}
	}
	return result
}

func planActionToTmuxAction(a plan.Action) tmux.Action {
	switch action := a.(type) {
	case plan.CreateSessionAction:
		return tmux.CreateSession{Name: action.Name, WindowName: action.WindowName, Path: action.Path}
	case plan.CreateWindowAction:
		return tmux.CreateWindow{Session: action.Session, Name: action.Name, Path: action.Path}
	case plan.SplitPaneAction:
		return tmux.SplitPane{Target: action.Target, Path: action.Path}
	case plan.SendKeysAction:
		return tmux.SendKeys{Target: action.Target, Keys: action.Command}
	case plan.SelectLayoutAction:
		return tmux.SelectLayout{Target: action.Target, Layout: action.Layout}
	case plan.ZoomPaneAction:
		return tmux.ZoomPane{Target: action.Target}
	case plan.KillSessionAction:
		return tmux.KillSession{Name: action.Name}
	case plan.KillWindowAction:
		return tmux.KillWindow{Target: action.Target}
	default:
		return nil
	}
}
