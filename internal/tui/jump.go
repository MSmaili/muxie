package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/MSmaili/hetki/internal/tui/list"
)

const jumpAlphabet = "asdfghjkl;qwertyuiopzxcvbnm"

type jumpCandidate struct {
	label  string
	itemID list.ItemID
}

type jumpState struct {
	candidates []jumpCandidate
	labels     map[list.ItemID]string
	input      string
}

func newJumpState(rows []list.Row) jumpState {
	ids := make([]list.ItemID, len(rows))
	for i, row := range rows {
		ids[i] = row.Item.ID
	}
	candidates := assignJumpLabels(ids)
	labels := make(map[list.ItemID]string, len(candidates))
	for _, candidate := range candidates {
		labels[candidate.itemID] = candidate.label
	}
	return jumpState{candidates: candidates, labels: labels}
}

func assignJumpLabels(ids []list.ItemID) []jumpCandidate {
	if len(ids) == 0 {
		return nil
	}
	alphabet := []rune(jumpAlphabet)
	width, capacity := 1, len(alphabet)
	for capacity < len(ids) {
		width++
		capacity *= len(alphabet)
	}
	candidates := make([]jumpCandidate, len(ids))
	for i, id := range ids {
		value := i
		label := make([]rune, width)
		for position := width - 1; position >= 0; position-- {
			label[position] = alphabet[value%len(alphabet)]
			value /= len(alphabet)
		}
		candidates[i] = jumpCandidate{label: string(label), itemID: id}
	}
	return candidates
}

func (j jumpState) labelFor(id list.ItemID) string {
	return j.labels[id]
}

func (j jumpState) neighbor(id list.ItemID, delta int) (list.ItemID, bool) {
	for i, candidate := range j.candidates {
		if candidate.itemID == id {
			return j.candidates[min(max(i+delta, 0), len(j.candidates)-1)].itemID, true
		}
	}
	return "", false
}

func (j *jumpState) enter(text string) (list.ItemID, bool, bool) {
	j.input += text
	validPrefix := false
	for _, candidate := range j.candidates {
		if candidate.label == j.input {
			return candidate.itemID, true, true
		}
		if len(candidate.label) > len(j.input) && candidate.label[:len(j.input)] == j.input {
			validPrefix = true
		}
	}
	if validPrefix {
		return "", false, true
	}
	j.input = ""
	return "", false, false
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
