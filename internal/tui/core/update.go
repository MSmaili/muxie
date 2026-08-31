package core

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/MSmaili/hetki/internal/tui/list"
	"github.com/charmbracelet/x/ansi"
)

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if m.mode == modeJump {
			m.cancelJump()
		}
		m.width, m.height = msg.Width, msg.Height
		m = m.reflow()
		m.startInitialJump()
		return m, nil
	case actionResultMsg:
		return m.handleActionResult(msg)
	case tea.PasteMsg:
		if m.busy {
			return m, nil
		}
		switch m.mode {
		case modeFilter:
			m.items.SetQuery(m.items.Query() + singleLinePaste(msg.Content))
			m.updateFilterStatus()
			return m.reflow(), nil
		case modeInput:
			m.input.Value += singleLinePaste(msg.Content)
		}
		return m, nil
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.busy {
			return m, nil
		}
		m.startInitialJump()
		switch m.mode {
		case modeJump:
			return m.updateJumpMode(msg)
		case modeFilter:
			return m.updateFilterMode(msg)
		case modeInput:
			return m.updateInputMode(msg)
		case modeConfirm:
			return m.updateConfirmMode(msg)
		case modeMenu:
			return m.updateMenuMode(msg)
		default:
			return m.updateBrowseMode(msg)
		}
	}
	return m, nil
}

func (m model) handleActionResult(msg actionResultMsg) (tea.Model, tea.Cmd) {
	previousRows := m.pendingRows
	m.pendingRows = nil
	if msg.result.Snapshot != nil && m.mode == modeJump {
		m.cancelJump()
	}
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
	if msg.result.Menu != nil {
		if pending == nil {
			m.err = fmt.Errorf("menu action has no pending request")
			m.status = m.err.Error()
			return m, nil
		}
		if err := validateItemMenu(*msg.result.Menu); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		title := strings.TrimSpace(msg.result.Menu.Title)
		if title == "" {
			title = "ACTIONS"
		}
		returnMode := primaryReturnMode(m.mode)
		m.mode = modeMenu
		m.menu = menuState{
			Title: title, ItemID: pending.ItemID,
			Entries:    append([]MenuEntry(nil), msg.result.Menu.Entries...),
			ReturnMode: returnMode,
		}
		return m.reflow(), nil
	}
	if msg.result.Input != nil {
		if pending == nil {
			m.err = fmt.Errorf("input action has no pending request")
			m.status = m.err.Error()
			return m, nil
		}
		returnMode := primaryReturnMode(m.mode)
		m.mode = modeInput
		m.input = inputState{
			Title: msg.result.Input.Title, Prompt: msg.result.Input.Prompt,
			Request: *pending, Value: msg.result.Input.InitialValue,
			SubmitStatus: msg.result.Input.SubmitStatus, ReturnMode: returnMode,
		}
		return m.reflow(), nil
	}
	if msg.result.Confirmation != nil {
		if pending == nil {
			m.err = fmt.Errorf("confirmation action has no pending request")
			m.status = m.err.Error()
			return m, nil
		}
		returnMode := primaryReturnMode(m.mode)
		m.mode = modeConfirm
		m.confirm = confirmState{
			Title: msg.result.Confirmation.Title, Body: msg.result.Confirmation.Body,
			Request: *pending, SubmitStatus: msg.result.Confirmation.SubmitStatus,
			ReturnMode: returnMode,
		}
		return m.reflow(), nil
	}
	if msg.result.Navigation != "" {
		m.navigation = msg.result.Navigation
		return m, tea.Quit
	}
	if msg.result.Snapshot != nil {
		if err := m.items.Replace(*msg.result.Snapshot, msg.result.SelectItemID); err != nil {
			m.err = err
			m.status = err.Error()
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
	position := 0
	for i, id := range previousRows {
		if id == deletedID {
			position = i
			break
		}
	}
	if m.selectSurvivor(previousRows, position) {
		return
	}
	if m.items.Query() != "" {
		m.items.SetQuery("")
		if m.selectSurvivor(previousRows, position) {
			return
		}
	}
	rows := m.items.Rows()
	if len(rows) > 0 {
		m.items.Select(rows[min(position, len(rows)-1)].Item.ID)
	}
}

func (m *model) selectSurvivor(previousRows []list.ItemID, position int) bool {
	for i := position + 1; i < len(previousRows); i++ {
		if m.items.Select(previousRows[i]) {
			return true
		}
	}
	for i := min(position-1, len(previousRows)-1); i >= 0; i-- {
		if m.items.Select(previousRows[i]) {
			return true
		}
	}
	return false
}

func (m model) updateBrowseMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.keys.Matches(KeyModeNormal, ActionQuit, msg):
		return m, tea.Quit
	case m.keys.Matches(KeyModeNormal, ActionMoveUp, msg):
		m.items.Move(-1)
	case m.keys.Matches(KeyModeNormal, ActionMoveDown, msg):
		m.items.Move(1)
	case m.keys.Matches(KeyModeNormal, ActionMoveTop, msg):
		m.items.Top()
	case m.keys.Matches(KeyModeNormal, ActionMoveBottom, msg):
		m.items.Bottom()
	case m.keys.Matches(KeyModeNormal, ActionPageUp, msg):
		m.items.Page(-1)
	case m.keys.Matches(KeyModeNormal, ActionPageDown, msg):
		m.items.Page(1)
	case m.keys.Matches(KeyModeNormal, ActionExpandAll, msg):
		if m.items.ExpandAll() {
			m.status = statusExpandedAll
		} else {
			m.status = statusExpandFiltered
		}
	case m.keys.Matches(KeyModeNormal, ActionCollapseAll, msg):
		if m.items.CollapseAll() {
			m.status = statusCollapsedAll
		} else {
			m.status = statusCollapseFiltered
		}
	case m.keys.Matches(KeyModeNormal, ActionNextMatch, msg):
		if m.items.JumpMatch(true) {
			m.status = matchJumpStatus(m)
		} else {
			m.status = statusNoMatches
		}
	case m.keys.Matches(KeyModeNormal, ActionPrevMatch, msg):
		if m.items.JumpMatch(false) {
			m.status = matchJumpStatus(m)
		} else {
			m.status = statusNoMatches
		}
	case m.keys.Matches(KeyModeNormal, ActionExpand, msg):
		if m.items.ToggleSelected(true) {
			m.status = statusExpanded
		} else {
			m.status = statusNothingToExpand
		}
	case m.keys.Matches(KeyModeNormal, ActionCollapse, msg):
		if m.items.ToggleSelected(false) {
			m.status = statusCollapsed
		} else {
			m.status = statusNothingToCollapse
		}
	case m.keys.Matches(KeyModeNormal, ActionFilter, msg):
		m.mode = modeFilter
		m.status = statusFilterHint
	case m.keys.Matches(KeyModeNormal, ActionJump, msg):
		m.jump = newJumpState(m.items.VisibleRows())
		if len(m.jump.candidates) == 0 {
			m.status = statusNoSelection
			return m, nil
		}
		m.mode = modeJump
		m.initialJump = false
		m.err = nil
	case m.keys.Matches(KeyModeNormal, ActionClearFilter, msg):
		m.items.SetQuery("")
		m.status = statusFilterCleared
	case m.keys.Matches(KeyModeNormal, ActionContextMenu, msg):
		return m.startSelectedRequest(ActionContextMenu)
	case m.keys.Matches(KeyModeNormal, ActionCreateSession, msg):
		return m.startAction(ActionCreateSession, "")
	case m.keys.Matches(KeyModeNormal, ActionCreateWindow, msg):
		return m.startSelectedRequest(ActionCreateWindow)
	case m.keys.Matches(KeyModeNormal, ActionRename, msg):
		return m.startSelectedRequest(ActionRename)
	case m.keys.Matches(KeyModeNormal, ActionDelete, msg):
		return m.startSelectedRequest(ActionDelete)
	case m.keys.Matches(KeyModeNormal, ActionRefresh, msg):
		return m.startAction(ActionRefresh, "")
	case m.keys.Matches(KeyModeNormal, ActionToggleProjection, msg):
		return m.startProjectionToggle()
	case m.keys.Matches(KeyModeNormal, ActionOpen, msg):
		return m.startSelectedRequest(ActionOpen)
	}
	return m.reflow(), nil
}

func (m model) updateJumpMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.keys.Matches(KeyModeJump, ActionCancel, msg):
		m.cancelJump()
		return m, nil
	case m.keys.Matches(KeyModeJump, ActionMoveUp, msg):
		m.moveJump(-1)
		return m, nil
	case m.keys.Matches(KeyModeJump, ActionMoveDown, msg):
		m.moveJump(1)
		return m, nil
	case m.keys.Matches(KeyModeJump, ActionFilter, msg):
		m.cancelJump()
		m.mode = modeFilter
		m.status = statusFilterHint
		return m, nil
	case m.keys.Matches(KeyModeJump, ActionToggleProjection, msg):
		return m.startProjectionToggle()
	}
	if msg.Text == "" {
		return m, nil
	}
	attempt := m.jump.input + msg.Text
	itemID, complete, valid := m.jump.enter(msg.Text)
	if !valid {
		m.err = fmt.Errorf("invalid jump label %q", attempt)
		m.status = m.err.Error()
		return m, nil
	}
	m.err = nil
	if !complete {
		return m, nil
	}
	m.mode = modeBrowse
	m.jump = jumpState{}
	return m.startAction(ActionOpen, itemID)
}

func (m *model) moveJump(delta int) {
	m.jump.input = ""
	m.err = nil
	selected, ok := m.items.Selected()
	if !ok {
		return
	}
	target, ok := m.jump.neighbor(selected.Item.ID, delta)
	if ok {
		m.items.Select(target)
	}
}

func (m *model) startInitialJump() {
	if !m.initialJump {
		return
	}
	m.initialJump = false
	m.mode = modeJump
	m.jump = newJumpState(m.items.VisibleRows())
}

func (m *model) cancelJump() {
	m.mode = modeBrowse
	m.jump = jumpState{}
	m.initialJump = false
	m.err = nil
}

func (m model) updateFilterMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.keys.Matches(KeyModeFilter, ActionCancel, msg):
		m.mode = modeBrowse
		m.status = statusFilterClosed
	case m.keys.Matches(KeyModeFilter, ActionConfirm, msg):
		selected, ok := m.selectedRow()
		m.mode = modeBrowse
		if !ok {
			m.status = statusFilterClosed
			return m, nil
		}
		return m.startAction(ActionOpen, selected.Item.ID)
	case m.keys.Matches(KeyModeFilter, ActionMoveUp, msg):
		m.items.Move(-1)
	case m.keys.Matches(KeyModeFilter, ActionMoveDown, msg):
		m.items.Move(1)
	case m.keys.Matches(KeyModeFilter, ActionPageUp, msg):
		m.items.Page(-1)
	case m.keys.Matches(KeyModeFilter, ActionPageDown, msg):
		m.items.Page(1)
	case m.keys.Matches(KeyModeFilter, ActionBackspace, msg):
		value := []rune(m.items.Query())
		if len(value) > 0 {
			m.items.SetQuery(string(value[:len(value)-1]))
		}
	case m.keys.Matches(KeyModeFilter, ActionDeleteWord, msg):
		m.items.SetQuery(deleteLastWord(m.items.Query()))
		m.updateFilterStatus()
	case m.keys.Matches(KeyModeFilter, ActionDeleteToStart, msg):
		m.items.SetQuery("")
		m.status = statusFilterHint
	case m.keys.Matches(KeyModeFilter, ActionClearFilter, msg):
		m.items.SetQuery("")
		m.mode = modeBrowse
		m.status = statusFilterCleared
	case m.keys.Matches(KeyModeFilter, ActionContextMenu, msg):
		return m.startSelectedRequest(ActionContextMenu)
	default:
		if msg.Text != "" {
			m.items.SetQuery(m.items.Query() + msg.Text)
			m.updateFilterStatus()
		}
	}
	return m.reflow(), nil
}

func (m model) updateMenuMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.keys.Matches(KeyModeMenu, ActionCancel, msg):
		m.mode = primaryReturnMode(m.menu.ReturnMode)
		m.menu = menuState{}
		m.status = statusActionCanceled
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
		if strings.EqualFold(msg.Text, string(entry.Activation)) {
			return m.activateMenuEntry(i)
		}
	}
	return m, nil
}

func (m model) activateMenuEntry(index int) (tea.Model, tea.Cmd) {
	if index < 0 || index >= len(m.menu.Entries) {
		m.err = fmt.Errorf("menu selection is unavailable")
		m.status = m.err.Error()
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
		m.status = statusActionCanceled
	case m.keys.Matches(KeyModeInput, ActionConfirm, msg):
		value := strings.TrimSpace(m.input.Value)
		if value == "" {
			m.status = statusValueEmpty
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
	case m.keys.Matches(KeyModeInput, ActionBackspace, msg):
		value := []rune(m.input.Value)
		if len(value) > 0 {
			m.input.Value = string(value[:len(value)-1])
		}
	case m.keys.Matches(KeyModeInput, ActionDeleteWord, msg):
		m.input.Value = deleteLastWord(m.input.Value)
	case m.keys.Matches(KeyModeInput, ActionDeleteToStart, msg):
		m.input.Value = ""
	default:
		if msg.Text != "" {
			m.input.Value += msg.Text
		}
	}
	return m, nil
}

func (m model) updateConfirmMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.keys.Matches(KeyModeConfirm, ActionCancel, msg):
		m.mode = primaryReturnMode(m.confirm.ReturnMode)
		m.confirm = confirmState{}
		m.status = statusActionCanceled
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
		m.status = statusNoSelection
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

func (m *model) updateFilterStatus() {
	if _, total := m.items.MatchPosition(); total > 0 {
		m.status = matchJumpStatus(*m)
	} else if strings.TrimSpace(m.items.Query()) == "" {
		m.status = statusFilterHint
	} else {
		m.status = statusNoMatches
	}
}

func matchJumpStatus(m model) string {
	current, total := m.items.MatchPosition()
	if total == 0 {
		return statusNoMatches
	}
	return fmt.Sprintf("match %d/%d", current, total)
}

func singleLinePaste(content string) string {
	content = ansi.Strip(strings.TrimRight(content, "\r\n"))
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\r' || r == '\n' || r == '\t':
			return ' '
		case r == utf8.RuneError || unicode.IsControl(r):
			return -1
		default:
			return r
		}
	}, content)
}

func deleteLastWord(value string) string {
	runes := []rune(value)
	end := len(runes)
	for end > 0 && (runes[end-1] == ' ' || runes[end-1] == '\t') {
		end--
	}
	for end > 0 && runes[end-1] != ' ' && runes[end-1] != '\t' {
		end--
	}
	return string(runes[:end])
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
