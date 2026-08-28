package tui

import (
	"encoding/base64"
	"fmt"
	"sort"

	"github.com/MSmaili/hetki/internal/backend"
	"github.com/MSmaili/hetki/internal/frecency"
	"github.com/MSmaili/hetki/internal/tui/list"
)

type pathGroup struct {
	path   string
	panes  []backend.Pane
	lowest int
}

func projectFlat(result backend.StateResult, homeDir string) (list.Snapshot, itemIndex, error) {
	return projectFlatRanked(result, homeDir, frecency.Scores{})
}

func projectFlatRanked(
	result backend.StateResult,
	homeDir string,
	scores frecency.Scores,
) (list.Snapshot, itemIndex, error) {
	snapshot := list.Snapshot{}
	index := make(itemIndex)

	for _, session := range result.Sessions {
		if err := appendFlatSession(&snapshot, index, session, result.Active.PaneID, homeDir); err != nil {
			return list.Snapshot{}, nil, err
		}
	}

	sort.Slice(snapshot.Items, func(i, j int) bool {
		left := index[snapshot.Items[i].ID]
		right := index[snapshot.Items[j].ID]
		return destinationLess(left, right, scores)
	})
	if err := validateProjection(snapshot, index); err != nil {
		return list.Snapshot{}, nil, err
	}
	return snapshot, index, nil
}

func appendFlatSession(
	snapshot *list.Snapshot,
	index itemIndex,
	session backend.Session,
	activePaneID string,
	homeDir string,
) error {
	if err := validateStableTmuxID(session.ID, '$', "session"); err != nil {
		return err
	}
	for _, window := range session.Windows {
		if err := appendFlatWindow(snapshot, index, session, window, activePaneID, homeDir); err != nil {
			return err
		}
	}
	return nil
}

func appendFlatWindow(
	snapshot *list.Snapshot,
	index itemIndex,
	session backend.Session,
	window backend.Window,
	activePaneID string,
	homeDir string,
) error {
	if err := validateStableTmuxID(window.ID, '@', "window"); err != nil {
		return err
	}
	for _, group := range groupPanesByPath(window.Panes) {
		item, destination, err := projectFlatDestination(session, window, group, homeDir)
		if err != nil {
			return err
		}
		snapshot.Items = append(snapshot.Items, item)
		index[item.ID] = destination
		if destination.Target == activePaneID {
			snapshot.ActiveItemID = item.ID
		}
	}
	return nil
}

func projectFlatDestination(
	session backend.Session,
	window backend.Window,
	group pathGroup,
	homeDir string,
) (list.Item, liveItem, error) {
	pane := destinationPane(group.panes)
	if err := validateStableTmuxID(pane.ID, '%', "pane"); err != nil {
		return list.Item{}, liveItem{}, err
	}

	name := window.Name
	if name == "" {
		name = fmt.Sprintf("%d", window.Index)
	}
	id := destinationItemID(session.ID, window.ID, group.path)
	fields := []list.SearchField{
		{Tier: list.SearchPrimary, Text: session.Name},
		{Tier: list.SearchPrimary, Text: name},
	}
	fields = appendPathSearchFields(fields, group.path, homeDir)

	item := list.Item{
		ID:           id,
		Primary:      session.Name + "" + name,
		Secondary:    displayPath(group.path, homeDir),
		SearchFields: fields,
	}
	destination := liveItem{
		ID:             id,
		SessionID:      list.ItemID("session:" + session.ID),
		WindowID:       list.ItemID("window:" + window.ID),
		Kind:           liveDestination,
		Label:          name,
		Name:           window.Name,
		SessionName:    session.Name,
		WindowName:     window.Name,
		Target:         pane.ID,
		MutationTarget: session.ID + ":" + window.ID,
		SessionTarget:  session.ID,
		RawPath:        group.path,
		WindowIndex:    window.Index,
		WindowActive:   window.Active,
		PaneActive:     pane.Active,
	}
	return item, destination, nil
}

func destinationLess(left, right liveItem, scores frecency.Scores) bool {
	leftScore := scores.Path(left.RawPath)
	rightScore := scores.Path(right.RawPath)
	if leftScore != rightScore {
		return leftScore > rightScore
	}

	leftScore = scores.Record(left.RawPath, left.SessionName)
	rightScore = scores.Record(right.RawPath, right.SessionName)
	if leftScore != rightScore {
		return leftScore > rightScore
	}
	if left.SessionName != right.SessionName {
		return left.SessionName < right.SessionName
	}
	if left.WindowIndex != right.WindowIndex {
		return left.WindowIndex < right.WindowIndex
	}
	if left.RawPath != right.RawPath {
		return left.RawPath < right.RawPath
	}
	if left.SessionTarget != right.SessionTarget {
		return left.SessionTarget < right.SessionTarget
	}
	return left.WindowID < right.WindowID
}

func appendPathSearchFields(fields []list.SearchField, rawPath, homeDir string) []list.SearchField {
	if rawPath != "" {
		fields = append(fields, list.SearchField{Tier: list.SearchSecondary, Text: rawPath})
	}
	display := displayPath(rawPath, homeDir)
	if display != "" && display != rawPath {
		fields = append(fields, list.SearchField{Tier: list.SearchSecondary, Text: display})
	}
	return fields
}

func groupPanesByPath(panes []backend.Pane) []pathGroup {
	groups := make([]pathGroup, 0, len(panes))
	byPath := make(map[string]int, len(panes))
	for _, pane := range panes {
		if i, exists := byPath[pane.Path]; exists {
			groups[i].panes = append(groups[i].panes, pane)
			if pane.Index < groups[i].lowest {
				groups[i].lowest = pane.Index
			}
			continue
		}
		byPath[pane.Path] = len(groups)
		groups = append(groups, pathGroup{path: pane.Path, panes: []backend.Pane{pane}, lowest: pane.Index})
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].lowest == groups[j].lowest {
			return groups[i].path < groups[j].path
		}
		return groups[i].lowest < groups[j].lowest
	})
	return groups
}

func destinationPane(panes []backend.Pane) backend.Pane {
	chosen := panes[0]
	for _, pane := range panes[1:] {
		if pane.Active != chosen.Active {
			if pane.Active {
				chosen = pane
			}
			continue
		}
		if pane.Index < chosen.Index {
			chosen = pane
		}
	}
	return chosen
}

func destinationItemID(sessionID, windowID, path string) list.ItemID {
	return list.ItemID("destination:" + sessionID + ":" + windowID + ":" + base64.RawURLEncoding.EncodeToString([]byte(path)))
}
