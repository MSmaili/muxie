# Hetki

A keyboard-first tmux workspace manager for Linux and macOS.

> **Alpha:** expect breaking changes. Hetki currently supports tmux only, and its behavior, appearance, and keybindings are not configurable yet.

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
