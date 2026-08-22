package core

import "github.com/MSmaili/hetki/internal/tui/list"

const jumpAlphabet = "asdfghjkl;qwertyuiopzxcvbnm"

type jumpCandidate struct {
	label  string
	itemID list.ItemID
}

type jumpState struct {
	candidates []jumpCandidate
	input      string
}

func newJumpState(rows []list.Row) jumpState {
	ids := make([]list.ItemID, len(rows))
	for i, row := range rows {
		ids[i] = row.Item.ID
	}
	return jumpState{candidates: assignJumpLabels(ids)}
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
	for _, candidate := range j.candidates {
		if candidate.itemID == id {
			return candidate.label
		}
	}
	return ""
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
