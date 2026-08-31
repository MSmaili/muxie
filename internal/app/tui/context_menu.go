package tui

import (
	"fmt"
	"strings"

	"github.com/MSmaili/hetki/internal/tui/core"
)

func contextualMenu(item liveItem) (core.ItemMenu, error) {
	title, openLabel, renameLabel, deleteLabel := "", "", "", ""
	switch item.Kind {
	case liveSession:
		title, openLabel, renameLabel, deleteLabel = "SESSION ACTIONS", "Open session", "Rename session", "Delete session"
	case liveWindow:
		title, openLabel, renameLabel, deleteLabel = "WINDOW ACTIONS", "Open window", "Rename window", "Delete window"
	case liveDestination:
		title, openLabel, renameLabel, deleteLabel = "DESTINATION ACTIONS", "Open destination", "Rename window", "Delete window"
	default:
		return core.ItemMenu{}, fmt.Errorf("item %q has an unknown kind", item.ID)
	}

	entries := make([]core.MenuEntry, 0, 9)
	if strings.TrimSpace(item.Target) != "" {
		entries = append(entries, core.MenuEntry{Action: core.ActionOpen, Label: openLabel, Activation: 'o'})
	}
	entries = append(entries, core.MenuEntry{Action: core.ActionCreateSession, Label: "New session", Activation: 's'})
	if strings.TrimSpace(item.SessionTarget) != "" {
		entries = append(entries, core.MenuEntry{Action: core.ActionCreateWindow, Label: "New window", Activation: 'w'})
	}
	if strings.TrimSpace(item.MutationTarget) != "" {
		entries = append(entries,
			core.MenuEntry{Action: core.ActionRename, Label: renameLabel, Activation: 'r'},
			core.MenuEntry{Action: core.ActionDelete, Label: deleteLabel, Activation: 'd'},
		)
	}
	if item.Kind == liveDestination && strings.TrimSpace(item.SessionTarget) != "" {
		entries = append(entries,
			core.MenuEntry{Action: core.ActionRenameSession, Label: "Rename session", Activation: 'n'},
			core.MenuEntry{Action: core.ActionDeleteSession, Label: "Delete session", Activation: 'x'},
		)
	}
	entries = append(entries,
		core.MenuEntry{Action: core.ActionRefresh, Label: "Refresh", Activation: 'f'},
		core.MenuEntry{Action: core.ActionToggleProjection, Label: "Toggle projection", Activation: 't'},
	)
	return core.ItemMenu{Title: title, Entries: entries}, nil
}
