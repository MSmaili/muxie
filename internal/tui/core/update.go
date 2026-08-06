package core

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/MSmaili/hetki/internal/tui/contracts"
)

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.reflow()
		return m, nil
	case actionResultMsg:
		return m.handleActionResult(msg)
	case tea.KeyPressMsg:
		// ctrl+c always quits, regardless of mode or overlay.
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// While the help overlay is open, only local keys close it.
		// Don't let global Quit or Help toggles fire on their own; esc/enter/? close it.
		if m.helpOpen {
			if key.Matches(msg, m.keys.Cancel) || key.Matches(msg, m.keys.Confirm) || key.Matches(msg, m.keys.Help) {
				m.helpOpen = false
				m.status = statusReady
			}
			return m, nil
		}

		if m.busy {
			return m, nil
		}

		// In modes that capture text input, do NOT honor the global Quit key (q).
		// Filter/input modes get first crack; confirm/browse still see the global binds.
		if m.mode == modeFilter {
			return m.updateFilterMode(msg)
		}
		if m.mode == modeInput {
			return m.updateInputMode(msg)
		}

		// Browse or confirm: global shortcuts are safe.
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}
		if key.Matches(msg, m.keys.Help) {
			m.helpOpen = true
			m.status = statusHelp
			return m, nil
		}

		if m.mode == modeConfirm {
			return m.updateConfirmMode(msg)
		}
		return m.updateBrowseMode(msg)
	}

	return m, nil
}

func (m model) handleActionResult(msg actionResultMsg) (tea.Model, tea.Cmd) {
	m.busy = false
	pending := m.pending
	m.pending = nil
	if msg.err != nil {
		m.err = msg.err
		m.status = msg.err.Error()
		return m, nil
	}
	m.err = nil
	if msg.result.Message != "" {
		m.status = msg.result.Message
	}
	// A successful switch means we're done — let the user get to their session.
	if pending != nil && pending.Type == contracts.IntentSwitch {
		return m, tea.Quit
	}
	if msg.result.Snapshot != nil {
		selectedID := ""
		if selected, ok := m.selectedRow(); ok {
			selectedID = selected.Node.ID
		}
		m.snapshot = *msg.result.Snapshot
		if m.expanded == nil {
			m.expanded = defaultExpanded(m.snapshot.Nodes, m.snapshot.ActiveNodeID)
		}
		markActivePathExpanded(m.snapshot.Nodes, m.snapshot.ActiveNodeID, m.expanded)
		m.applyFilter()
		m = m.reflow()
		preferredID := preferredSelectionID(m.snapshot, pending, selectedID)
		if idx := findRowIndexByID(m.rows, preferredID); idx >= 0 {
			m.cursor = idx
		} else if idx := findRowIndexByID(m.rows, selectedID); idx >= 0 {
			m.cursor = idx
		}
		m = m.reflow()
	}
	return m, nil
}

func (m model) updateBrowseMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.cursor = clampCursor(m.cursor-1, len(m.rows))
		m = m.reflow()
		return m, nil
	case key.Matches(msg, m.keys.Down):
		m.cursor = clampCursor(m.cursor+1, len(m.rows))
		m = m.reflow()
		return m, nil
	case key.Matches(msg, m.keys.Top):
		m.cursor = clampCursor(0, len(m.rows))
		m = m.reflow()
		return m, nil
	case key.Matches(msg, m.keys.Bottom):
		m.cursor = clampCursor(len(m.rows)-1, len(m.rows))
		m = m.reflow()
		return m, nil
	case key.Matches(msg, m.keys.PageUp):
		m.cursor = clampCursor(m.cursor-m.listH, len(m.rows))
		m = m.reflow()
		return m, nil
	case key.Matches(msg, m.keys.PageDown):
		m.cursor = clampCursor(m.cursor+m.listH, len(m.rows))
		m = m.reflow()
		return m, nil
	case key.Matches(msg, m.keys.ExpandAll):
		if m.expandAll() {
			m.status = statusExpandedAll
		} else {
			m.status = statusExpandFiltered
		}
		return m, nil
	case key.Matches(msg, m.keys.CollapseAll):
		if m.collapseAll() {
			m.status = statusCollapsedAll
		} else {
			m.status = statusCollapseFiltered
		}
		return m, nil
	case key.Matches(msg, m.keys.NextMatch):
		if m.jumpMatch(true) {
			m.status = matchJumpStatus(m)
		} else {
			m.status = statusNoMatches
		}
		return m, nil
	case key.Matches(msg, m.keys.PrevMatch):
		if m.jumpMatch(false) {
			m.status = matchJumpStatus(m)
		} else {
			m.status = statusNoMatches
		}
		return m, nil
	case key.Matches(msg, m.keys.Expand):
		if m.toggleCurrentRow(true) {
			m.status = statusExpanded
		} else {
			m.status = statusNothingToExpand
		}
		return m, nil
	case key.Matches(msg, m.keys.Collapse):
		if m.toggleCurrentRow(false) {
			m.status = statusCollapsed
		} else {
			m.status = statusNothingToCollapse
		}
		return m, nil
	case key.Matches(msg, m.keys.Search):
		m.mode = modeFilter
		m.status = statusFilterHint
		return m, nil
	case key.Matches(msg, m.keys.ClearFilter):
		m.filter = ""
		m.applyFilter()
		m = m.reflow()
		m.status = statusFilterCleared
		return m, nil
	case key.Matches(msg, m.keys.CreateSession):
		var ok bool
		if m, ok = m.requireCapability(contracts.CapabilityCreateSession, "create session"); !ok {
			return m, nil
		}
		m.mode = modeInput
		m.input = inputState{
			Title:        "CREATE SESSION",
			Prompt:       "Session name",
			IntentType:   contracts.IntentCreateSession,
			SubmitStatus: submitCreatingSession,
		}
		m.status = statusEnterSessionName
		return m.reflow(), nil
	case key.Matches(msg, m.keys.CreateWindow):
		var ok bool
		if m, ok = m.requireCapability(contracts.CapabilityCreateWindow, "create window"); !ok {
			return m, nil
		}
		session := m.selectedSessionTarget()
		if session == "" {
			m.status = statusSelectSessionHint
			return m, nil
		}
		m.mode = modeInput
		m.input = inputState{
			Title:      "CREATE WINDOW",
			Prompt:     "Window name",
			IntentType: contracts.IntentCreateWindow,
			Payload: map[string]string{
				"session": session,
			},
			SubmitStatus: submitCreatingWindow,
		}
		m.status = statusEnterWindowName
		return m.reflow(), nil
	case key.Matches(msg, m.keys.Rename):
		return m.beginRenameFlow()
	case key.Matches(msg, m.keys.Delete):
		return m.beginDeleteFlow()
	case key.Matches(msg, m.keys.Refresh):
		var ok bool
		if m, ok = m.requireCapability(contracts.CapabilityRefresh, "refresh"); !ok {
			return m, nil
		}
		m.busy = true
		m.status = statusRefreshing
		return m, runIntent(m.dispatch, contracts.Intent{Type: contracts.IntentRefresh})
	case key.Matches(msg, m.keys.Confirm):
		var ok bool
		if m, ok = m.requireCapability(contracts.CapabilitySwitch, "switch"); !ok {
			return m, nil
		}
		selected, ok := m.selectedRow()
		if !ok {
			m.status = statusNoSelection
			return m, nil
		}
		if selected.Node.Target == "" {
			m.status = statusNotActionable
			return m, nil
		}
		switchIntent := contracts.Intent{
			Type:   contracts.IntentSwitch,
			Target: selected.Node.Target,
		}
		m.busy = true
		m.pending = cloneIntent(switchIntent)
		m.status = statusSwitching
		return m, runIntent(m.dispatch, switchIntent)
	}

	return m, nil
}

func (m model) updateFilterMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.mode = modeBrowse
		m.status = statusFilterClosed
		return m, nil
	case key.Matches(msg, m.keys.Confirm):
		if !m.hasCapability(contracts.CapabilitySwitch) {
			m.mode = modeBrowse
			m.status = statusFilterClosed
			return m, nil
		}
		selected, ok := m.selectedRow()
		if !ok || selected.Node.Target == "" {
			m.mode = modeBrowse
			m.status = statusFilterClosed
			return m, nil
		}
		m.mode = modeBrowse
		switchIntent := contracts.Intent{
			Type:   contracts.IntentSwitch,
			Target: selected.Node.Target,
		}
		m.busy = true
		m.pending = cloneIntent(switchIntent)
		m.status = statusSwitching
		return m, runIntent(m.dispatch, switchIntent)
	case msg.String() == "up", msg.String() == "ctrl+p":
		m.cursor = clampCursor(m.cursor-1, len(m.rows))
		return m.reflow(), nil
	case msg.String() == "down", msg.String() == "ctrl+n":
		m.cursor = clampCursor(m.cursor+1, len(m.rows))
		return m.reflow(), nil
	case msg.String() == "pgup":
		m.cursor = clampCursor(m.cursor-m.listH, len(m.rows))
		return m.reflow(), nil
	case msg.String() == "pgdown":
		m.cursor = clampCursor(m.cursor+m.listH, len(m.rows))
		return m.reflow(), nil
	case key.Matches(msg, m.keys.Backspace):
		if len(m.filter) > 0 {
			r := []rune(m.filter)
			m.filter = string(r[:len(r)-1])
			m.applyFilter()
			m = m.reflow()
		}
		return m, nil
	case key.Matches(msg, m.keys.DeleteWord):
		m.filter = deleteLastWord(m.filter)
		m.applyFilter()
		m = m.reflow()
		if _, total := m.filterMatchPosition(); total > 0 {
			m.status = matchJumpStatus(m)
		} else if strings.TrimSpace(m.filter) == "" {
			m.status = statusFilterHint
		} else {
			m.status = statusNoMatches
		}
		return m, nil
	case key.Matches(msg, m.keys.DeleteToStart):
		m.filter = ""
		m.applyFilter()
		m = m.reflow()
		m.status = statusFilterHint
		return m, nil
	case key.Matches(msg, m.keys.ClearFilter):
		m.filter = ""
		m.applyFilter()
		m.mode = modeBrowse
		m.status = statusFilterCleared
		return m.reflow(), nil
	}

	if len(msg.Text) > 0 {
		m.filter += msg.Text
		m.applyFilter()
		if _, total := m.filterMatchPosition(); total > 0 {
			m.status = matchJumpStatus(m)
		} else {
			m.status = statusNoMatches
		}
		return m.reflow(), nil
	}

	return m, nil
}

func (m model) updateInputMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.mode = modeBrowse
		m.input = inputState{}
		m.status = statusActionCanceled
		return m.reflow(), nil
	case key.Matches(msg, m.keys.Confirm):
		value := strings.TrimSpace(m.input.Value)
		if value == "" {
			m.status = statusValueEmpty
			return m, nil
		}

		intent := contracts.Intent{
			Type:    m.input.IntentType,
			Target:  m.input.Target,
			Payload: clonePayloadMap(m.input.Payload),
		}
		if intent.Payload == nil {
			intent.Payload = make(map[string]string)
		}
		intent.Payload["name"] = value

		m.mode = modeBrowse
		m.busy = true
		m.pending = cloneIntent(intent)
		if strings.TrimSpace(m.input.SubmitStatus) != "" {
			m.status = m.input.SubmitStatus
		} else {
			m.status = statusRunningAction
		}
		m.input = inputState{}
		return m.reflow(), runIntent(m.dispatch, intent)
	case key.Matches(msg, m.keys.Backspace):
		if len(m.input.Value) == 0 {
			return m, nil
		}
		r := []rune(m.input.Value)
		m.input.Value = string(r[:len(r)-1])
		return m, nil
	case key.Matches(msg, m.keys.DeleteWord):
		m.input.Value = deleteLastWord(m.input.Value)
		return m, nil
	case key.Matches(msg, m.keys.DeleteToStart):
		m.input.Value = ""
		return m, nil
	}

	if len(msg.Text) > 0 {
		m.input.Value += msg.Text
	}

	return m, nil
}

func (m model) updateConfirmMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := strings.ToLower(strings.TrimSpace(msg.String()))
	switch {
	case key.Matches(msg, m.keys.Cancel), k == "n":
		m.mode = modeBrowse
		m.confirm = confirmState{}
		m.status = statusActionCanceled
		return m.reflow(), nil
	case key.Matches(msg, m.keys.Confirm), k == "y":
		intent := m.confirm.Intent
		status := strings.TrimSpace(m.confirm.SubmitStatus)
		if status == "" {
			status = statusRunningAction
		}
		m.mode = modeBrowse
		m.confirm = confirmState{}
		m.busy = true
		m.pending = cloneIntent(intent)
		m.status = status
		return m.reflow(), runIntent(m.dispatch, intent)
	}
	return m, nil
}

func (m model) beginDeleteFlow() (tea.Model, tea.Cmd) {
	if len(m.rows) == 0 {
		m.status = statusNoSelection
		return m, nil
	}
	selected, ok := m.selectedRow()
	if !ok {
		m.status = statusNoSelection
		return m, nil
	}

	switch selected.Node.Kind {
	case contracts.NodeKindSession:
		var ok bool
		if m, ok = m.requireCapability(contracts.CapabilityDeleteSession, "delete session"); !ok {
			return m, nil
		}
		m.mode = modeConfirm
		m.confirm = confirmState{
			Title: "DELETE SESSION",
			Body:  fmt.Sprintf("Delete session %q?", selected.Node.Label),
			Intent: contracts.Intent{
				Type:   contracts.IntentDeleteSession,
				Target: selected.Node.Target,
			},
			SubmitStatus: submitDeletingSession,
		}
		m.status = statusConfirmDelete
		return m.reflow(), nil
	case contracts.NodeKindWindow:
		var ok bool
		if m, ok = m.requireCapability(contracts.CapabilityDeleteWindow, "delete window"); !ok {
			return m, nil
		}
		m.mode = modeConfirm
		m.confirm = confirmState{
			Title: "DELETE WINDOW",
			Body:  fmt.Sprintf("Delete window %q?", selected.Node.Label),
			Intent: contracts.Intent{
				Type:   contracts.IntentDeleteWindow,
				Target: selected.Node.Target,
			},
			SubmitStatus: submitDeletingWindow,
		}
		m.status = statusConfirmDelete
		return m.reflow(), nil
	default:
		m.status = statusDeleteOnlyKinds
		return m, nil
	}
}

func (m model) beginRenameFlow() (tea.Model, tea.Cmd) {
	selected, ok := m.selectedRow()
	if !ok {
		m.status = statusNoSelection
		return m, nil
	}

	switch selected.Node.Kind {
	case contracts.NodeKindSession:
		var ok bool
		if m, ok = m.requireCapability(contracts.CapabilityRenameSession, "rename session"); !ok {
			return m, nil
		}
		m.mode = modeInput
		m.input = inputState{
			Title:        "RENAME SESSION",
			Prompt:       "Session name",
			IntentType:   contracts.IntentRenameSession,
			Target:       selected.Node.Target,
			Value:        renameInitialValue(selected),
			SubmitStatus: submitRenamingSession,
		}
		m.status = statusEnterNewSession
		return m.reflow(), nil
	case contracts.NodeKindWindow:
		var ok bool
		if m, ok = m.requireCapability(contracts.CapabilityRenameWindow, "rename window"); !ok {
			return m, nil
		}
		m.mode = modeInput
		m.input = inputState{
			Title:        "RENAME WINDOW",
			Prompt:       "Window name",
			IntentType:   contracts.IntentRenameWindow,
			Target:       selected.Node.Target,
			Value:        renameInitialValue(selected),
			SubmitStatus: submitRenamingWindow,
		}
		m.status = statusEnterNewWindow
		return m.reflow(), nil
	default:
		m.status = statusRenameOnlyKinds
		return m, nil
	}
}

func (m model) selectedSessionTarget() string {
	selected, ok := m.selectedRow()
	if !ok {
		return ""
	}
	return sessionFromNodeTarget(selected.Node.Target)
}

func renameInitialValue(selected row) string {
	label := strings.TrimSpace(selected.Node.Label)
	if selected.Node.Kind != contracts.NodeKindWindow {
		return label
	}
	parts := strings.Fields(label)
	if len(parts) <= 1 {
		return label
	}
	return strings.Join(parts[1:], " ")
}

func matchJumpStatus(m model) string {
	current, total := m.filterMatchPosition()
	if total == 0 {
		return "no matches"
	}
	return fmt.Sprintf("match %d/%d", current, total)
}

func deleteLastWord(value string) string {
	r := []rune(value)
	end := len(r)
	for end > 0 && (r[end-1] == ' ' || r[end-1] == '\t') {
		end--
	}
	for end > 0 && r[end-1] != ' ' && r[end-1] != '\t' {
		end--
	}
	return string(r[:end])
}

func runIntent(dispatch DispatchFunc, intent contracts.Intent) tea.Cmd {
	if dispatch == nil {
		return nil
	}
	return func() tea.Msg {
		result, err := dispatch(context.Background(), intent)
		return actionResultMsg{result: result, err: err}
	}
}
