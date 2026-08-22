package core

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

type KeyMode string

const (
	KeyModeNormal  KeyMode = "normal"
	KeyModeJump    KeyMode = "jump"
	KeyModeFilter  KeyMode = "filter"
	KeyModeInput   KeyMode = "input"
	KeyModeConfirm KeyMode = "confirm"
)

type Binding struct {
	Action ActionID
	Keys   []string
}

type KeyMap struct {
	bindings map[KeyMode]map[ActionID]key.Binding
}

func ResolveKeyMap(bindings map[KeyMode][]Binding) (KeyMap, error) {
	resolved := KeyMap{bindings: make(map[KeyMode]map[ActionID]key.Binding, len(bindings))}
	for mode, modeBindings := range bindings {
		actions := make(map[ActionID]key.Binding, len(modeBindings))
		used := make(map[string]ActionID)
		for _, binding := range modeBindings {
			if binding.Action == "" || len(binding.Keys) == 0 {
				return KeyMap{}, fmt.Errorf("mode %q has an empty action or key list", mode)
			}
			if _, exists := actions[binding.Action]; exists {
				return KeyMap{}, fmt.Errorf("mode %q defines action %q more than once", mode, binding.Action)
			}
			keys := make([]string, len(binding.Keys))
			for i, raw := range binding.Keys {
				value := strings.TrimSpace(raw)
				if value == "" {
					return KeyMap{}, fmt.Errorf("mode %q action %q has an empty key", mode, binding.Action)
				}
				if value == "ctrl+c" {
					return KeyMap{}, fmt.Errorf("ctrl+c is reserved for emergency quit")
				}
				if previous, exists := used[value]; exists {
					return KeyMap{}, fmt.Errorf("mode %q key %q is assigned to %q and %q", mode, value, previous, binding.Action)
				}
				used[value] = binding.Action
				keys[i] = value
			}
			actions[binding.Action] = key.NewBinding(key.WithKeys(keys...))
		}
		resolved.bindings[mode] = actions
	}
	return resolved, nil
}

func DefaultKeyMap() KeyMap {
	keymap, err := ResolveKeyMap(map[KeyMode][]Binding{
		KeyModeNormal: {
			{Action: ActionQuit, Keys: []string{"q"}},
			{Action: ActionMoveUp, Keys: []string{"up", "k", "ctrl+p"}},
			{Action: ActionMoveDown, Keys: []string{"down", "j", "ctrl+n"}},
			{Action: ActionMoveTop, Keys: []string{"g"}},
			{Action: ActionMoveBottom, Keys: []string{"G"}},
			{Action: ActionPageUp, Keys: []string{"pgup", "u"}},
			{Action: ActionPageDown, Keys: []string{"pgdown", "d"}},
			{Action: ActionFilter, Keys: []string{"/"}},
			{Action: ActionJump, Keys: []string{";"}},
			{Action: ActionNextMatch, Keys: []string{"n"}},
			{Action: ActionPrevMatch, Keys: []string{"N"}},
			{Action: ActionClearFilter, Keys: []string{"ctrl+l"}},
			{Action: ActionCreateSession, Keys: []string{"s"}},
			{Action: ActionCreateWindow, Keys: []string{"a"}},
			{Action: ActionRename, Keys: []string{"e"}},
			{Action: ActionDelete, Keys: []string{"x"}},
			{Action: ActionExpand, Keys: []string{"right", "l"}},
			{Action: ActionCollapse, Keys: []string{"left", "h"}},
			{Action: ActionExpandAll, Keys: []string{"L"}},
			{Action: ActionCollapseAll, Keys: []string{"H"}},
			{Action: ActionRefresh, Keys: []string{"r"}},
			{Action: ActionToggleProjection, Keys: []string{"tab"}},
			{Action: ActionOpen, Keys: []string{"enter", "ctrl+y"}},
		},
		KeyModeJump: {
			{Action: ActionCancel, Keys: []string{"esc"}},
			{Action: ActionMoveUp, Keys: []string{"ctrl+p"}},
			{Action: ActionMoveDown, Keys: []string{"ctrl+n"}},
			{Action: ActionFilter, Keys: []string{"/"}},
		},
		KeyModeFilter: {
			{Action: ActionCancel, Keys: []string{"esc"}},
			{Action: ActionConfirm, Keys: []string{"enter", "ctrl+y"}},
			{Action: ActionMoveUp, Keys: []string{"up", "ctrl+p"}},
			{Action: ActionMoveDown, Keys: []string{"down", "ctrl+n"}},
			{Action: ActionPageUp, Keys: []string{"pgup"}},
			{Action: ActionPageDown, Keys: []string{"pgdown"}},
			{Action: ActionBackspace, Keys: []string{"backspace", "delete"}},
			{Action: ActionDeleteWord, Keys: []string{"ctrl+w"}},
			{Action: ActionDeleteToStart, Keys: []string{"ctrl+u"}},
			{Action: ActionClearFilter, Keys: []string{"ctrl+l"}},
		},
		KeyModeInput: {
			{Action: ActionCancel, Keys: []string{"esc"}},
			{Action: ActionConfirm, Keys: []string{"enter", "ctrl+y"}},
			{Action: ActionBackspace, Keys: []string{"backspace", "delete"}},
			{Action: ActionDeleteWord, Keys: []string{"ctrl+w"}},
			{Action: ActionDeleteToStart, Keys: []string{"ctrl+u"}},
		},
		KeyModeConfirm: {
			{Action: ActionCancel, Keys: []string{"esc", "n", "N"}},
			{Action: ActionConfirm, Keys: []string{"enter", "ctrl+y", "y", "Y"}},
		},
	})
	if err != nil {
		panic(err)
	}
	return keymap
}

func (k KeyMap) IsZero() bool { return k.bindings == nil }

func (k KeyMap) Keys(mode KeyMode, action ActionID) []string {
	binding, ok := k.bindings[mode][action]
	if !ok {
		return nil
	}
	return append([]string(nil), binding.Keys()...)
}

func (k KeyMap) Matches(mode KeyMode, action ActionID, msg tea.KeyPressMsg) bool {
	binding, ok := k.bindings[mode][action]
	return ok && key.Matches(msg, binding)
}
