package core

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/MSmaili/hetki/internal/tui/list"
)

type ActionID string

const (
	ActionQuit             ActionID = "quit"
	ActionMoveUp           ActionID = "move_up"
	ActionMoveDown         ActionID = "move_down"
	ActionMoveTop          ActionID = "move_top"
	ActionMoveBottom       ActionID = "move_bottom"
	ActionPageUp           ActionID = "page_up"
	ActionPageDown         ActionID = "page_down"
	ActionFilter           ActionID = "filter"
	ActionJump             ActionID = "jump"
	ActionNextMatch        ActionID = "next_match"
	ActionPrevMatch        ActionID = "previous_match"
	ActionClearFilter      ActionID = "clear_filter"
	ActionContextMenu      ActionID = "context_menu"
	ActionCreateSession    ActionID = "create_session"
	ActionCreateWindow     ActionID = "create_window"
	ActionRename           ActionID = "rename"
	ActionRenameSession    ActionID = "rename_session"
	ActionDelete           ActionID = "delete"
	ActionDeleteSession    ActionID = "delete_session"
	ActionExpand           ActionID = "expand"
	ActionCollapse         ActionID = "collapse"
	ActionExpandAll        ActionID = "expand_all"
	ActionCollapseAll      ActionID = "collapse_all"
	ActionBackspace        ActionID = "backspace"
	ActionDeleteWord       ActionID = "delete_word"
	ActionDeleteToStart    ActionID = "delete_to_start"
	ActionCancel           ActionID = "cancel"
	ActionConfirm          ActionID = "confirm"
	ActionRefresh          ActionID = "refresh"
	ActionToggleProjection ActionID = "toggle_projection"
	ActionOpen             ActionID = "open"
)

type BackendTarget string

type ActionRequest struct {
	ActionID  ActionID
	ItemID    list.ItemID
	Value     *string
	Confirmed bool
}

type InputPrompt struct {
	Title        string
	Prompt       string
	InitialValue string
	SubmitStatus string
}

type Confirmation struct {
	Title        string
	Body         string
	SubmitStatus string
}

type MenuEntry struct {
	Action     ActionID
	Label      string
	Activation rune
}

type ItemMenu struct {
	Title   string
	Entries []MenuEntry
}

func validateItemMenu(menu ItemMenu) error {
	if len(menu.Entries) == 0 {
		return fmt.Errorf("selected item has no available actions")
	}
	actions := make(map[ActionID]struct{}, len(menu.Entries))
	activations := make(map[rune]struct{}, len(menu.Entries))
	for _, entry := range menu.Entries {
		if entry.Action == "" || strings.TrimSpace(entry.Label) == "" || !unicode.IsLetter(entry.Activation) {
			return fmt.Errorf("menu entry has an invalid action, label, or activation letter")
		}
		if _, exists := actions[entry.Action]; exists {
			return fmt.Errorf("menu action %q appears more than once", entry.Action)
		}
		activation := unicode.ToLower(entry.Activation)
		if _, exists := activations[activation]; exists {
			return fmt.Errorf("menu activation %q appears more than once", entry.Activation)
		}
		actions[entry.Action] = struct{}{}
		activations[activation] = struct{}{}
	}
	return nil
}

type ActionResult struct {
	Message      string
	Snapshot     *list.Snapshot
	SelectItemID list.ItemID
	Input        *InputPrompt
	Confirmation *Confirmation
	Menu         *ItemMenu
	Navigation   BackendTarget
}
