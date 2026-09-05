package tui

import (
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/MSmaili/hetki/internal/tui/list"
)

type DispatchFunc func(ActionRequest) (ActionResult, error)

type actionResultMsg struct {
	result ActionResult
	err    error
}

func (m model) handleActionResult(msg actionResultMsg) (tea.Model, tea.Cmd) {
	previousRows := m.pendingRows
	m.pendingRows = nil
	if msg.result.Snapshot != nil && m.mode == modeJump {
		m.cancelJump()
	}
	m.busy = false
	m.status = ""
	pending := m.pending
	m.pending = nil
	if msg.err != nil {
		m.err = msg.err
		return m, nil
	}
	m.err = nil
	switch {
	case msg.result.Menu != nil || msg.result.Input != nil || msg.result.Confirmation != nil:
		m.openOverlay(msg.result, pending)
		return m.reflow(), nil
	case msg.result.Navigation != "":
		m.navigation = msg.result.Navigation
		return m, tea.Quit
	}
	if msg.result.Snapshot != nil {
		if err := m.items.Replace(*msg.result.Snapshot, msg.result.SelectItemID); err != nil {
			m.err = err
			return m, nil
		}
		if pending != nil && isDeleteAction(pending.ActionID) {
			m.selectAfterDelete(pending.ItemID, msg.result.SelectItemID, previousRows)
		}
		m = m.reflow()
	}
	return m, nil
}

func isDeleteAction(action ActionID) bool {
	return action == ActionDelete || action == ActionDeleteSession
}

func (m *model) selectAfterDelete(deletedID, preferred list.ItemID, previousRows []list.ItemID) {
	if preferred != "" {
		if m.items.Select(preferred) {
			return
		}
		if m.items.Query() != "" {
			m.items.SetQuery("")
			if m.items.Select(preferred) {
				return
			}
		}
	}
	if m.items.Select(deletedID) {
		return
	}
	if m.items.SelectSurvivor(previousRows, deletedID) {
		return
	}
	if m.items.Query() != "" {
		m.items.SetQuery("")
		if m.items.SelectSurvivor(previousRows, deletedID) {
			return
		}
	}
	rows := m.items.Rows()
	if len(rows) > 0 {
		position := max(0, slices.Index(previousRows, deletedID))
		m.items.Select(rows[min(position, len(rows)-1)].Item.ID)
	}
}

func (m model) startProjectionToggle() (tea.Model, tea.Cmd) {
	var itemID list.ItemID
	if selected, ok := m.selectedRow(); ok {
		itemID = selected.Item.ID
	}
	return m.startAction(ActionToggleProjection, itemID)
}

func (m model) startSelectedRequest(action ActionID) (tea.Model, tea.Cmd) {
	selected, ok := m.selectedRow()
	if !ok {
		return m, nil
	}
	return m.startAction(action, selected.Item.ID)
}

func (m model) startAction(action ActionID, itemID list.ItemID) (tea.Model, tea.Cmd) {
	status := statusRunningAction
	switch action {
	case ActionOpen:
		status = statusSwitching
	case ActionRefresh:
		status = statusRefreshing
	}
	return m.startRequest(ActionRequest{ActionID: action, ItemID: itemID}, status)
}

func (m model) startRequest(request ActionRequest, status string) (tea.Model, tea.Cmd) {
	m.busy = true
	m.pending = cloneRequest(request)
	m.pendingRows = nil
	if isDeleteAction(request.ActionID) && request.Confirmed {
		rows := m.items.Rows()
		m.pendingRows = make([]list.ItemID, len(rows))
		for i, row := range rows {
			m.pendingRows[i] = row.Item.ID
		}
	}
	m.status = status
	return m, runAction(m.dispatch, request)
}

func runAction(dispatch DispatchFunc, request ActionRequest) tea.Cmd {
	if dispatch == nil {
		return nil
	}
	return func() tea.Msg {
		result, err := dispatch(request)
		return actionResultMsg{result: result, err: err}
	}
}

func cloneRequest(request ActionRequest) *ActionRequest {
	cloned := request
	return &cloned
}
