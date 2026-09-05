package update

import (
	"context"
	"fmt"
	"os"

	"github.com/MSmaili/hetki/internal/logger"
)

const (
	modulePath = "github.com/MSmaili/hetki"
	githubRepo = "MSmaili/hetki"
)

type Options struct {
	CurrentVersion  string
	TargetVersion   string // exact tag from --version; empty means latest
	AllowPrerelease bool   // --pre; prereleases are otherwise invisible
	FromSource      bool
	DryRun          bool
	Verbose         bool
}

type Target struct {
	Tag    string
	Commit string
}

type Updater interface {
	Name() string
	Update(context.Context, Target) error
	DryRun(Target)
}

type Service struct {
	SetVerbose       func(bool)
	Executable       func() (string, error)
	DetermineUpdater func(string) (Updater, error)
	ResolveTarget    func(context.Context, Options) (string, error)
	ResolveCommit    func(context.Context, string) (string, error)
}

func NewService() Service {
	return Service{}
}

func (s Service) Run(ctx context.Context, opts Options) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.setVerbose(opts.Verbose)

	exePath, err := s.executable()
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}
	logger.Debug("Executable path: %s", exePath)

	updater, err := s.determineUpdater(exePath, opts)
	if err != nil {
		return err
	}
	logger.Verbose("Detected installation method: %s", updater.Name())

	// D4: fail closed — no update proceeds without an exact resolved tag.
	targetTag, err := s.resolveTarget(ctx, opts)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("could not resolve a release to install: %w", err)
	}
	exact := opts.TargetVersion != ""
	install, reason, err := decideUpdate(opts.CurrentVersion, targetTag, exact, opts.AllowPrerelease)
	if err != nil {
		return err
	}
	if !install {
		logger.Success("%s (%s)", reason, opts.CurrentVersion)
		return nil
	}

	commit, err := s.resolveCommit(ctx, targetTag)
	if err != nil {
		return fmt.Errorf("could not resolve commit for %s: %w", targetTag, err)
	}
	target := Target{Tag: targetTag, Commit: commit}
	if opts.DryRun {
		updater.DryRun(target)
		return nil
	}
	if opts.CurrentVersion == "dev" {
		logger.Info("Development build detected, updating to: %s", targetTag)
	} else {
		logger.Info("Current version: %s", opts.CurrentVersion)
		logger.Info("Updating to: %s (%s)", targetTag, reason)
	}
	if err := updater.Update(ctx, target); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	logger.Success("Update completed successfully")
	return nil
}

func (s Service) setVerbose(verbose bool) {
	if s.SetVerbose != nil {
		s.SetVerbose(verbose)
		return
	}
	logger.SetVerbose(verbose)
}

func (s Service) executable() (string, error) {
	if s.Executable != nil {
		return s.Executable()
	}
	return os.Executable()
}

func (s Service) determineUpdater(exePath string, opts Options) (Updater, error) {
	if s.DetermineUpdater != nil {
		return s.DetermineUpdater(exePath)
	}
	return DetermineUpdater(exePath, opts)
}

func (s Service) resolveTarget(ctx context.Context, opts Options) (string, error) {
	if s.ResolveTarget != nil {
		return s.ResolveTarget(ctx, opts)
	}
	return ResolveTarget(ctx, opts)
}

func (s Service) resolveCommit(ctx context.Context, tag string) (string, error) {
	if s.ResolveCommit != nil {
		return s.ResolveCommit(ctx, tag)
	}
	return resolveTagCommit(ctx, tag)
}
