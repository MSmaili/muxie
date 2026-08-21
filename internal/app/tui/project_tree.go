package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/MSmaili/hetki/internal/backend"
	"github.com/MSmaili/hetki/internal/tui/list"
)

const (
	sessionIcon = "\U000f018d"
	windowIcon  = "\ueb14"
)

func projectTree(result backend.StateResult, homeDir string) (list.Snapshot, itemIndex, error) {
	snapshot := list.Snapshot{Items: make([]list.Item, 0, len(result.Sessions))}
	index := make(itemIndex)
	for _, session := range result.Sessions {
		sessionTarget := session.ID
		if err := validateStableTmuxID(sessionTarget, '$', "session"); err != nil {
			return list.Snapshot{}, nil, err
		}
		sessionID := list.ItemID("session:" + sessionTarget)
		sessionItem := list.Item{
			ID: sessionID, Primary: sessionIcon + " " + session.Name,
			SearchFields: []list.SearchField{{Tier: list.SearchPrimary, Text: session.Name}},
		}
		index[sessionID] = liveItem{
			ID: sessionID, SessionID: sessionID, Kind: liveSession, Label: session.Name, Name: session.Name,
			SessionName: session.Name, Target: sessionTarget, MutationTarget: sessionTarget, SessionTarget: sessionTarget,
		}
		activeSession := result.Active.SessionID == session.ID || (result.Active.SessionID == "" && result.Active.Session == session.Name)
		if activeSession {
			snapshot.ActiveItemID = sessionID
		}

		for _, window := range session.Windows {
			windowTarget := window.ID
			if err := validateStableTmuxID(windowTarget, '@', "window"); err != nil {
				return list.Snapshot{}, nil, err
			}
			windowID := list.ItemID("window:" + windowTarget)
			label := fmt.Sprintf("%d", window.Index)
			if window.Name != "" {
				label += " " + window.Name
			}
			path := displayPath(window.Path, homeDir)
			fields := []list.SearchField{{Tier: list.SearchPrimary, Text: label}}
			if path != "" {
				fields = append(fields, list.SearchField{Tier: list.SearchSecondary, Text: path})
			}
			itemID := windowID
			sessionItem.Children = append(sessionItem.Children, list.Item{
				ID: itemID, Primary: windowIcon + " " + label, Secondary: path, SearchFields: fields,
			})
			mutationTarget := sessionTarget + ":" + windowTarget
			index[itemID] = liveItem{
				ID: itemID, ParentID: sessionID, SessionID: sessionID, WindowID: windowID, Kind: liveWindow,
				Label: label, Name: window.Name, SessionName: session.Name, WindowName: window.Name,
				Target: mutationTarget, MutationTarget: mutationTarget, SessionTarget: sessionTarget,
				WindowIndex: window.Index, WindowActive: window.Active,
			}
			activeWindow := result.Active.WindowID == window.ID || (result.Active.WindowID == "" && result.Active.WindowIndex == window.Index)
			if activeSession && activeWindow {
				snapshot.ActiveItemID = itemID
			}
		}
		snapshot.Items = append(snapshot.Items, sessionItem)
	}
	if err := validateProjection(snapshot, index); err != nil {
		return list.Snapshot{}, nil, err
	}
	return snapshot, index, nil
}

func validateStableTmuxID(value string, prefix byte, kind string) error {
	text := strings.TrimPrefix(value, string(prefix))
	id, err := strconv.Atoi(text)
	if !strings.HasPrefix(value, string(prefix)) || err != nil || id < 0 || strconv.Itoa(id) != text {
		return fmt.Errorf("%s must have a stable %cN ID, got %q", kind, prefix, value)
	}
	return nil
}
