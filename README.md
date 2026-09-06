# Hetki

A keyboard-first tmux workspace manager for Linux and macOS.

> **Alpha:** expect breaking changes. Hetki currently supports tmux only, and its behavior, appearance, and keybindings are not configurable yet.

<img width="736" height="432" alt="image" src="https://github.com/user-attachments/assets/69ed76a6-8690-486d-8a60-1543d11f219b" />


## What it does

- Browse and switch between tmux sessions and windows in an interactive TUI.
- Save the current session, or all sessions, as a strict YAML workspace.
- Start and reconcile saved workspaces.

## Installation

Hetki requires tmux and, on macOS, Ventura 13 or later.

### Using Go

```bash
go install github.com/MSmaili/hetki@latest
```

### Using curl

```bash
curl -fsSL https://raw.githubusercontent.com/MSmaili/hetki/main/install.sh | bash
```

## Updating

```bash
hetki update          # Latest stable release
hetki update --head   # Latest pushed commit on main (requires Go; unreleased code)
```

`--head` pins and verifies the current `main` commit before building. Repeat it to
get newer pushed commits; local unpushed changes are not included.
Release updates exclude prereleases. Use `--version vX.Y.Z` to return to a specific
stable release.

## Quick start

```bash
hetki                 # Open the TUI
hetki save -n work    # Save the currently attached tmux session
hetki start work      # Start or reconcile the saved workspace
hetki switch dev      # Switch directly to a session
```

Run `hetki --help` or `hetki <command> --help` for the current CLI options.

## Safety

`hetki start --force` reconciles windows only inside sessions declared by the selected workspace; it never removes unrelated sessions. Saves use atomic replacement and refuse to overwrite a destination that changed while it was being saved.
