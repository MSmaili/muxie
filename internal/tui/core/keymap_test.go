package core

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveKeyMapRejectsSameModeCollisionsAndReservedQuit(t *testing.T) {
	_, err := ResolveKeyMap(map[KeyMode][]Binding{KeyModeNormal: {
		{Action: ActionQuit, Keys: []string{"q"}},
		{Action: ActionRefresh, Keys: []string{"q"}},
	}})
	require.ErrorContains(t, err, "is assigned")

	_, err = ResolveKeyMap(map[KeyMode][]Binding{KeyModeNormal: {
		{Action: ActionQuit, Keys: []string{"ctrl+c"}},
	}})
	require.ErrorContains(t, err, "reserved")
}

func TestModelUsesTheInjectedResolvedKeyMap(t *testing.T) {
	keys, err := ResolveKeyMap(map[KeyMode][]Binding{KeyModeNormal: {
		{Action: ActionRefresh, Keys: []string{"z"}},
	}})
	require.NoError(t, err)
	var got ActionRequest
	m, err := newModelWithKeys(interactionSnapshot(), func(request ActionRequest) (ActionResult, error) {
		got = request
		return ActionResult{}, nil
	}, keys)
	require.NoError(t, err)
	m.mode = modeBrowse

	m, cmd := updateModel(t, m, printableKey("z"))
	require.True(t, m.busy)
	require.NotNil(t, cmd)
	cmd()
	require.Equal(t, ActionRequest{ActionID: ActionRefresh}, got)
}

func TestDefaultConfirmationBindingsPreserveUppercaseChoices(t *testing.T) {
	keys := DefaultKeyMap()
	require.Contains(t, keys.Keys(KeyModeConfirm, ActionConfirm), "Y")
	require.Contains(t, keys.Keys(KeyModeConfirm, ActionCancel), "N")
}

func TestModalControlsUseTheResolvedBindings(t *testing.T) {
	keys, err := ResolveKeyMap(map[KeyMode][]Binding{KeyModeInput: {
		{Action: ActionConfirm, Keys: []string{"z"}},
		{Action: ActionCancel, Keys: []string{"x"}},
	}})
	require.NoError(t, err)
	require.Equal(t, "z submit | x cancel", modalControls(keys, KeyModeInput, "submit"))
}

func TestResolveKeyMapAllowsTheSameKeyInDifferentModes(t *testing.T) {
	_, err := ResolveKeyMap(map[KeyMode][]Binding{
		KeyModeNormal: {{Action: ActionQuit, Keys: []string{"q"}}},
		KeyModeFilter: {{Action: ActionCancel, Keys: []string{"q"}}},
	})
	require.NoError(t, err)
}
