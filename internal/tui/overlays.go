package tui

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/MSmaili/hetki/internal/tui/list"
)

type inputState struct {
	Title        string
	Prompt       string
	Request      ActionRequest
	Value        string
	SubmitStatus string
	ReturnMode   uiMode
}

type confirmState struct {
	Title        string
	Body         string
	Request      ActionRequest
	SubmitStatus string
	ReturnMode   uiMode
}

type menuState struct {
	Title      string
	ItemID     list.ItemID
	Entries    []MenuEntry
	Cursor     int
	ReturnMode uiMode
}

func (m *model) openOverlay(result ActionResult, pending *ActionRequest) {
	if pending == nil {
		m.err = fmt.Errorf("overlay action has no pending request")
		return
	}
	returnMode := primaryReturnMode(m.mode)
	switch {
	case result.Menu != nil:
		menu := result.Menu
		if err := validateItemMenu(*menu, m.keys); err != nil {
			m.err = err
			return
		}
		title := strings.TrimSpace(menu.Title)
		if title == "" {
			title = "ACTIONS"
		}
		m.mode = modeMenu
		m.menu = menuState{
			Title: title, ItemID: pending.ItemID,
			Entries: append([]MenuEntry(nil), menu.Entries...), ReturnMode: returnMode,
		}
	case result.Input != nil:
		prompt := result.Input
		m.mode = modeInput
		m.input = inputState{
			Title: prompt.Title, Prompt: prompt.Prompt, Request: *pending,
			Value: prompt.InitialValue, SubmitStatus: prompt.SubmitStatus, ReturnMode: returnMode,
		}
	case result.Confirmation != nil:
		confirmation := result.Confirmation
		m.mode = modeConfirm
		m.confirm = confirmState{
			Title: confirmation.Title, Body: confirmation.Body, Request: *pending,
			SubmitStatus: confirmation.SubmitStatus, ReturnMode: returnMode,
		}
	}
}

func (m model) updateMenuMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.keys.Matches(KeyModeMenu, ActionCancel, msg):
		m.mode = primaryReturnMode(m.menu.ReturnMode)
		m.menu = menuState{}
		return m, nil
	case m.keys.Matches(KeyModeMenu, ActionMoveUp, msg):
		m.menu.Cursor = max(0, m.menu.Cursor-1)
		return m, nil
	case m.keys.Matches(KeyModeMenu, ActionMoveDown, msg):
		m.menu.Cursor = min(len(m.menu.Entries)-1, m.menu.Cursor+1)
		return m, nil
	case m.keys.Matches(KeyModeMenu, ActionConfirm, msg):
		return m.activateMenuEntry(m.menu.Cursor)
	}
	for i, entry := range m.menu.Entries {
		if m.keys.Matches(m.keys.menuMode(entry.Action), entry.Action, msg) {
			return m.activateMenuEntry(i)
		}
	}
	return m, nil
}

func (m model) activateMenuEntry(index int) (tea.Model, tea.Cmd) {
	if index < 0 || index >= len(m.menu.Entries) {
		m.err = fmt.Errorf("menu selection is unavailable")
		return m, nil
	}
	entry := m.menu.Entries[index]
	itemID := m.menu.ItemID
	m.mode = primaryReturnMode(m.menu.ReturnMode)
	m.menu = menuState{}
	return m.startAction(entry.Action, itemID)
}

func (m model) updateInputMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.keys.Matches(KeyModeInput, ActionCancel, msg):
		m.mode = primaryReturnMode(m.input.ReturnMode)
		m.input = inputState{}
		m.err = nil
		return m, nil
	case m.keys.Matches(KeyModeInput, ActionConfirm, msg):
		value := strings.TrimSpace(m.input.Value)
		if value == "" {
			m.err = errors.New(statusValueEmpty)
			return m, nil
		}
		request := m.input.Request
		request.Value = &value
		status := m.input.SubmitStatus
		if strings.TrimSpace(status) == "" {
			status = statusRunningAction
		}
		m.mode = primaryReturnMode(m.input.ReturnMode)
		m.input = inputState{}
		return m.startRequest(request, status)
	}
	value, edited := editEndOfLine(m.input.Value, m.keys, KeyModeInput, msg)
	if edited {
		m.input.Value = value
		m.err = nil
	}
	return m, nil
}

func (m model) updateConfirmMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.keys.Matches(KeyModeConfirm, ActionCancel, msg):
		m.mode = primaryReturnMode(m.confirm.ReturnMode)
		m.confirm = confirmState{}
	case m.keys.Matches(KeyModeConfirm, ActionConfirm, msg):
		request := m.confirm.Request
		request.Confirmed = true
		status := m.confirm.SubmitStatus
		if strings.TrimSpace(status) == "" {
			status = statusRunningAction
		}
		m.mode = primaryReturnMode(m.confirm.ReturnMode)
		m.confirm = confirmState{}
		return m.startRequest(request, status)
	}
	return m, nil
}

func primaryReturnMode(mode uiMode) uiMode {
	if mode == modeFilter {
		return modeFilter
	}
	return modeBrowse
}

func validateItemMenu(menu ItemMenu, keys KeyMap) error {
	if len(menu.Entries) == 0 {
		return fmt.Errorf("selected item has no available actions")
	}
	used := map[string]ActionID{"ctrl+c": ActionQuit}
	for _, action := range []ActionID{ActionCancel, ActionConfirm, ActionMoveUp, ActionMoveDown} {
		for _, bound := range keys.Keys(KeyModeMenu, action) {
			used[bound] = action
		}
	}
	actions := make(map[ActionID]struct{}, len(menu.Entries))
	for _, entry := range menu.Entries {
		if entry.Action == "" || strings.TrimSpace(entry.Label) == "" {
			return fmt.Errorf("menu entry has an invalid action or label")
		}
		if _, exists := actions[entry.Action]; exists {
			return fmt.Errorf("menu action %q appears more than once", entry.Action)
		}
		for _, bound := range keys.Keys(keys.menuMode(entry.Action), entry.Action) {
			if previous, exists := used[bound]; exists {
				return fmt.Errorf("menu key %q is assigned to %q and %q", bound, previous, entry.Action)
			}
			used[bound] = entry.Action
		}
		actions[entry.Action] = struct{}{}
	}
	return nil
}
