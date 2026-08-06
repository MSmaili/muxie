package start

import (
	"fmt"
	"path/filepath"
	"strings"

	appshared "github.com/MSmaili/hetki/internal/app"
	"github.com/MSmaili/hetki/internal/backend"
	"github.com/MSmaili/hetki/internal/converter"
	"github.com/MSmaili/hetki/internal/logger"
	"github.com/MSmaili/hetki/internal/manifest"
	"github.com/MSmaili/hetki/internal/plan"
	"github.com/MSmaili/hetki/internal/state"
)

type Options struct {
	Workspace string
	DryRun    bool
	Force     bool
}

type Service struct {
	DetectBackend func(...string) (backend.Backend, error)
	LoadWorkspace func(string) (*manifest.Workspace, string, error)
}

func NewService(detectBackend func(...string) (backend.Backend, error)) Service {
	return Service{DetectBackend: detectBackend}
}

func (s Service) Run(opts Options) error {
	workspace, workspacePath, err := s.loadWorkspace(opts.Workspace)
	if err != nil {
		return err
	}

	b, err := s.detectBackend()
	if err != nil {
		return fmt.Errorf("failed to detect backend: %w", err)
	}

	p, err := buildPlan(b, workspace, opts.Force)
	if err != nil {
		return err
	}

	return executePlan(b, p, workspace, workspacePath, opts.DryRun)
}

func (s Service) loadWorkspace(nameOrPath string) (*manifest.Workspace, string, error) {
	if s.LoadWorkspace != nil {
		return s.LoadWorkspace(nameOrPath)
	}
	return appshared.NewWorkspaceLoader().LoadWorkspace(nameOrPath)
}

func (s Service) detectBackend() (backend.Backend, error) {
	if s.DetectBackend != nil {
		return s.DetectBackend()
	}
	return backend.Detect()
}

func buildPlan(b backend.Backend, workspace *manifest.Workspace, force bool) (*plan.Plan, error) {
	desired := converter.ManifestToState(workspace)

	result, err := b.QueryState()
	if err != nil {
		return nil, fmt.Errorf("failed to query backend state: %w\nHint: Verify tmux is running and retry, or inspect live sessions with 'hetki list sessions'", err)
	}
	actual := converter.BackendResultToState(result)

	diff := state.Compare(desired, actual)
	planDiff := converter.StateDiffToPlanDiff(diff, desired)

	strategy := selectStrategy(force)
	p := strategy.Plan(planDiff)
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("invalid reconciliation plan: %w", err)
	}
	return p, nil
}

func selectStrategy(force bool) plan.Strategy {
	if force {
		return &plan.ForceStrategy{}
	}
	return &plan.MergeStrategy{}
}

func executePlan(b backend.Backend, p *plan.Plan, workspace *manifest.Workspace, workspacePath string, dryRun bool) error {
	if p.IsEmpty() {
		logger.Info("Workspace already up to date")
		warnIfMetadataStampFails(b, workspace, workspacePath)
		return attachToSession(b, workspace)
	}

	if dryRun {
		printDryRun(b, p)
		return nil
	}

	if err := b.Apply(toBackendActions(p.Actions)); err != nil {
		return fmt.Errorf("failed to execute plan: %w\nHint: Check tmux server logs or try with --dry-run to see planned actions", err)
	}

	warnIfMetadataStampFails(b, workspace, workspacePath)

	return attachToSession(b, workspace)
}

func warnIfMetadataStampFails(b backend.Backend, workspace *manifest.Workspace, workspacePath string) {
	if err := stampWorkspacePath(b, workspace, workspacePath); err != nil {
		logger.Warning("workspace metadata not set: %v", err)
	}
}

func stampWorkspacePath(b backend.Backend, workspace *manifest.Workspace, workspacePath string) error {
	path := strings.TrimSpace(workspacePath)
	if path == "" {
		return nil
	}
	if absPath, err := filepath.Abs(path); err == nil {
		path = absPath
	}

	actions := make([]backend.Action, 0, len(workspace.Sessions))
	for _, session := range workspace.Sessions {
		if strings.TrimSpace(session.Name) == "" {
			continue
		}
		actions = append(actions, backend.SetSessionOptionAction{
			Session: session.Name,
			Key:     backend.WorkspacePathOption,
			Value:   path,
		})
	}
	if len(actions) == 0 {
		return nil
	}
	if err := b.Apply(actions); err != nil {
		return err
	}
	return nil
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
		result[i] = toBackendAction(a)
	}
	return result
}

func toBackendAction(action plan.Action) backend.Action {
	switch a := action.(type) {
	case plan.CreateSessionAction:
		return backend.CreateSessionAction{Name: a.Name, WindowName: a.WindowName, Path: a.Path}
	case plan.CreateWindowAction:
		return backend.CreateWindowAction{Session: a.Session, Name: a.Name, Path: a.Path}
	case plan.SplitPaneAction:
		return backend.SplitPaneAction{Session: a.Session, Window: a.Window, Path: a.Path}
	case plan.SendKeysAction:
		return backend.SendKeysAction{Session: a.Session, Window: a.Window, Pane: a.Pane, Command: a.Command}
	case plan.KillSessionAction:
		return backend.KillSessionAction{Name: a.Name}
	case plan.KillWindowAction:
		return backend.KillWindowAction{Session: a.Session, Window: a.Window, WindowID: a.WindowID}
	case plan.SelectLayoutAction:
		return backend.SelectLayoutAction{Session: a.Session, Window: a.Window, Layout: a.Layout}
	case plan.ZoomPaneAction:
		return backend.ZoomPaneAction{Session: a.Session, Window: a.Window, Pane: a.Pane}
	default:
		return nil
	}
}

func attachToSession(b backend.Backend, workspace *manifest.Workspace) error {
	if len(workspace.Sessions) > 0 {
		return b.Attach(workspace.Sessions[0].Name)
	}
	return nil
}
