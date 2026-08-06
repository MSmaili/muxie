package core

// Status messages shown in the status bar. Centralized here to avoid
// typos and keep wording consistent.
const (
	statusReady             = "ready"
	statusHelp              = "help"
	statusNoSelection       = "no selection"
	statusNoMatches         = "no matches"
	statusValueEmpty        = "value cannot be empty"
	statusActionCanceled    = "action canceled"
	statusRunningAction     = "running action..."
	statusRefreshing        = "refreshing..."
	statusSwitching         = "switching..."
	statusFilterClosed      = "filter closed"
	statusFilterCleared     = "filter cleared"
	statusFilterHint        = "type to filter, enter switch, esc close"
	statusExpanded          = "expanded"
	statusCollapsed         = "collapsed"
	statusExpandedAll       = "expanded all"
	statusCollapsedAll      = "collapsed all"
	statusNothingToExpand   = "nothing to expand"
	statusNothingToCollapse = "nothing to collapse"
	statusExpandFiltered    = "expand unavailable while filtering"
	statusCollapseFiltered  = "collapse unavailable while filtering"
	statusEnterSessionName  = "enter session name"
	statusEnterWindowName   = "enter window name"
	statusEnterNewSession   = "enter new session name"
	statusEnterNewWindow    = "enter new window name"
	statusSelectSessionHint = "select a session, window, or pane first"
	statusNotActionable     = "selected item is not actionable"
	statusConfirmDelete     = "confirm delete"
	statusDeleteOnlyKinds   = "delete supports sessions/windows only"
	statusRenameOnlyKinds   = "rename supports sessions/windows only"

	// Submit-status messages shown while a backend action is in flight.
	submitCreatingSession = "creating session..."
	submitCreatingWindow  = "creating window..."
	submitDeletingSession = "deleting session..."
	submitDeletingWindow  = "deleting window..."
	submitRenamingSession = "renaming session..."
	submitRenamingWindow  = "renaming window..."
)
