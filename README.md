# Hetki - Your smart terminal multiplexer sidekick

A smart terminal multiplexer session manager (supports tmux)

`hetki start --force` reconciles windows only inside sessions declared by the selected workspace. It never removes unrelated sessions.

Saves use optimistic conflict detection: if the destination changes while being saved, Hetki leaves the newer bytes untouched and asks you to retry.

## Installation

### Using Go

```bash
go install github.com/MSmaili/hetki@latest
```

### Using curl (Linux/macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/MSmaili/hetki/main/install.sh | bash
```
