package contracts

type NodeKind string

const (
	NodeKindSession NodeKind = "session"
	NodeKindWindow  NodeKind = "window"
	NodeKindPane    NodeKind = "pane"
)

type NodeID string
type BackendTarget string

type Capability string

const (
	CapabilityRefresh Capability = "refresh"
	CapabilitySwitch  Capability = "switch"

	CapabilityCreateSession Capability = "create_session"
	CapabilityCreateWindow  Capability = "create_window"
	CapabilityRenameSession Capability = "rename_session"
	CapabilityRenameWindow  Capability = "rename_window"
	CapabilityDeleteSession Capability = "delete_session"
	CapabilityDeleteWindow  Capability = "delete_window"
	CapabilityMoveWindow    Capability = "move_window"
	CapabilityPersist       Capability = "persist_changes"
)

type Node struct {
	ID       NodeID
	ParentID NodeID
	Kind     NodeKind
	Label    string
	Name     string
	Target   BackendTarget
	Path     string
	Active   bool
	Children []Node
}

type Snapshot struct {
	Nodes        []Node
	ActiveNodeID NodeID
	Workspace    string
	Capabilities map[Capability]bool
}

type IntentType string

const (
	IntentRefresh IntentType = "refresh"
	IntentSwitch  IntentType = "switch"

	IntentCreateSession IntentType = "create_session"
	IntentCreateWindow  IntentType = "create_window"
	IntentRenameSession IntentType = "rename_session"
	IntentRenameWindow  IntentType = "rename_window"
	IntentDeleteSession IntentType = "delete_session"
	IntentDeleteWindow  IntentType = "delete_window"
	IntentMoveWindow    IntentType = "move_window"
	IntentPersist       IntentType = "persist_changes"
)

type Intent struct {
	Type         IntentType
	NodeID       NodeID
	ParentNodeID NodeID
	Target       BackendTarget
	Session      BackendTarget
	Name         string
}

type ActionResult struct {
	Message      string
	Snapshot     *Snapshot
	NeedsRefresh bool
	Navigation   BackendTarget
}
