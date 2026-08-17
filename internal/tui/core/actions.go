package core

import "github.com/MSmaili/hetki/internal/tui/list"

type ActionID string

const (
	ActionQuit          ActionID = "quit"
	ActionMoveUp        ActionID = "move_up"
	ActionMoveDown      ActionID = "move_down"
	ActionMoveTop       ActionID = "move_top"
	ActionMoveBottom    ActionID = "move_bottom"
	ActionPageUp        ActionID = "page_up"
	ActionPageDown      ActionID = "page_down"
	ActionFilter        ActionID = "filter"
	ActionJump          ActionID = "jump"
	ActionNextMatch     ActionID = "next_match"
	ActionPrevMatch     ActionID = "previous_match"
	ActionClearFilter   ActionID = "clear_filter"
	ActionCreateSession ActionID = "create_session"
	ActionCreateWindow  ActionID = "create_window"
	ActionRename        ActionID = "rename"
	ActionDelete        ActionID = "delete"
	ActionExpand        ActionID = "expand"
	ActionCollapse      ActionID = "collapse"
	ActionExpandAll     ActionID = "expand_all"
	ActionCollapseAll   ActionID = "collapse_all"
	ActionBackspace     ActionID = "backspace"
	ActionDeleteWord    ActionID = "delete_word"
	ActionDeleteToStart ActionID = "delete_to_start"
	ActionCancel        ActionID = "cancel"
	ActionConfirm       ActionID = "confirm"
	ActionRefresh       ActionID = "refresh"
	ActionOpen          ActionID = "open"
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

type ActionResult struct {
	Message      string
	Snapshot     *list.Snapshot
	SelectItemID list.ItemID
	Input        *InputPrompt
	Confirmation *Confirmation
	Navigation   BackendTarget
}
