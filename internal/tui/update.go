package tui

import tea "charm.land/bubbletea/v2"

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
			return m.reflow(), nil
		case modeInput:
			m.input.Value += singleLinePaste(msg.Content)
			m.err = nil
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
		m.items.ExpandAll()
	case m.keys.Matches(KeyModeNormal, ActionCollapseAll, msg):
		m.items.CollapseAll()
	case m.keys.Matches(KeyModeNormal, ActionNextMatch, msg):
		m.items.JumpMatch(true)
	case m.keys.Matches(KeyModeNormal, ActionPrevMatch, msg):
		m.items.JumpMatch(false)
	case m.keys.Matches(KeyModeNormal, ActionExpand, msg):
		m.items.ToggleSelected(true)
	case m.keys.Matches(KeyModeNormal, ActionCollapse, msg):
		m.items.ToggleSelected(false)
	case m.keys.Matches(KeyModeNormal, ActionFilter, msg):
		m.mode = modeFilter
	case m.keys.Matches(KeyModeNormal, ActionJump, msg):
		m.jump = newJumpState(m.items.VisibleRows())
		if len(m.jump.candidates) == 0 {
			return m, nil
		}
		m.mode = modeJump
		m.initialJump = false
		m.err = nil
	case m.keys.Matches(KeyModeNormal, ActionClearFilter, msg):
		m.items.SetQuery("")
	case m.keys.Matches(KeyModeNormal, ActionContextMenu, msg):
		return m.startSelectedRequest(ActionContextMenu)
	case m.keys.Matches(KeyModeNormal, ActionCreateSession, msg):
		return m.startAction(ActionCreateSession, "")
	case m.keys.Matches(KeyModeNormal, ActionCreateWindow, msg):
		return m.startSelectedRequest(ActionCreateWindow)
	case m.keys.Matches(KeyModeNormal, ActionRename, msg):
		return m.startSelectedRequest(ActionRename)
	case m.keys.Matches(KeyModeNormal, ActionRenameSession, msg):
		return m.startSelectedRequest(ActionRenameSession)
	case m.keys.Matches(KeyModeNormal, ActionDelete, msg):
		return m.startSelectedRequest(ActionDelete)
	case m.keys.Matches(KeyModeNormal, ActionDeleteSession, msg):
		return m.startSelectedRequest(ActionDeleteSession)
	case m.keys.Matches(KeyModeNormal, ActionRefresh, msg):
		return m.startAction(ActionRefresh, "")
	case m.keys.Matches(KeyModeNormal, ActionToggleProjection, msg):
		return m.startProjectionToggle()
	case m.keys.Matches(KeyModeNormal, ActionOpen, msg):
		return m.startSelectedRequest(ActionOpen)
	}
	return m.reflow(), nil
}

func (m model) updateFilterMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.keys.Matches(KeyModeFilter, ActionCancel, msg):
		m.mode = modeBrowse
	case m.keys.Matches(KeyModeFilter, ActionConfirm, msg):
		selected, ok := m.selectedRow()
		m.mode = modeBrowse
		if !ok {
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
	case m.keys.Matches(KeyModeFilter, ActionClearFilter, msg):
		m.items.SetQuery("")
		m.mode = modeBrowse
	case m.keys.Matches(KeyModeFilter, ActionContextMenu, msg):
		return m.startSelectedRequest(ActionContextMenu)
	default:
		if value, edited := editEndOfLine(m.items.Query(), m.keys, KeyModeFilter, msg); edited {
			m.items.SetQuery(value)
		}
	}
	return m.reflow(), nil
}
