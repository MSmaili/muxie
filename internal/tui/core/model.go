package core

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/MSmaili/hetki/internal/tui/contracts"
)

type DispatchFunc func(contracts.Intent) (contracts.ActionResult, error)

func Run(ctx context.Context, initial contracts.Snapshot, dispatch DispatchFunc) (contracts.BackendTarget, error) {
	p := tea.NewProgram(newModel(initial, dispatch), tea.WithContext(ctx))
	final, err := p.Run()
	if err != nil {
		return "", err
	}
	m, ok := final.(model)
	if !ok {
		return "", fmt.Errorf("unexpected final TUI model %T", final)
	}
	return m.navigation, nil
}

type row struct {
	Node       contracts.Node
	Depth      int
	TreePrefix string
	Expanded   bool
	Branch     bool
	// score is the filter match score: > 0 when the row itself matches the
	// active query, 0 for ancestor-only rows and when no filter is active.
	score int
}

type actionResultMsg struct {
	result contracts.ActionResult
	err    error
}

type uiMode string

const (
	modeBrowse  uiMode = "browse"
	modeFilter  uiMode = "filter"
	modeInput   uiMode = "input"
	modeConfirm uiMode = "confirm"
)

type inputState struct {
	Title        string
	Prompt       string
	IntentType   contracts.IntentType
	NodeID       contracts.NodeID
	Target       contracts.BackendTarget
	Session      contracts.BackendTarget
	Value        string
	SubmitStatus string
}

type confirmState struct {
	Title        string
	Body         string
	Intent       contracts.Intent
	SubmitStatus string
}

type model struct {
	snapshot   contracts.Snapshot
	rows       []row
	cursor     int
	offset     int
	listH      int
	mode       uiMode
	filter     string
	status     string
	err        error
	busy       bool
	input      inputState
	confirm    confirmState
	expanded   map[contracts.NodeID]bool
	pending    *contracts.Intent
	navigation contracts.BackendTarget
	helpOpen   bool

	width  int
	height int

	dispatch DispatchFunc
	keys     KeyMap
	theme    theme
}

func newModel(snapshot contracts.Snapshot, dispatch DispatchFunc) model {
	m := model{
		snapshot: snapshot,
		dispatch: dispatch,
		keys:     DefaultKeyMap(),
		theme:    defaultTheme(),
		mode:     modeFilter,
		status:   statusFilterHint,
		expanded: defaultExpanded(snapshot.Nodes, snapshot.ActiveNodeID),
	}
	m.applyFilter()
	m.cursor = clampCursor(0, len(m.rows))
	return m.reflow()
}

func clampCursor(cursor, size int) int {
	if size == 0 {
		return 0
	}
	if cursor < 0 {
		return 0
	}
	if cursor >= size {
		return size - 1
	}
	return cursor
}

func (m model) reflow() model {
	m.cursor = clampCursor(m.cursor, len(m.rows))
	m.listH = m.availableListHeight()
	m.offset = clampOffset(m.offset, len(m.rows), m.listH)
	m.offset = ensureCursorVisible(m.offset, m.cursor, len(m.rows), m.listH)
	return m
}

type layoutMetrics struct {
	lineWidth    int
	innerWidth   int
	middleHeight int
	compact      bool
	frameStyle   lipgloss.Style
}

func (m model) layout() layoutMetrics {
	lineWidth := m.width
	if lineWidth <= 0 {
		lineWidth = 100
	}
	height := m.height
	if height <= 0 {
		height = 24
	}
	frameStyle := responsiveFrameStyle(m.theme.appBorder, lineWidth)
	innerWidth := max(1, lineWidth-frameStyle.GetHorizontalFrameSize())
	return layoutMetrics{
		lineWidth:    lineWidth,
		innerWidth:   innerWidth,
		middleHeight: max(1, height-frameStyle.GetVerticalFrameSize()-2),
		compact:      innerWidth < 56,
		frameStyle:   frameStyle,
	}
}

func (m model) availableListHeight() int { return m.layout().middleHeight }

func clampOffset(offset, total, height int) int {
	if total <= 0 || height <= 0 {
		return 0
	}
	maxOffset := total - height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset < 0 {
		return 0
	}
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}

func ensureCursorVisible(offset, cursor, total, height int) int {
	if total <= 0 || height <= 0 {
		return 0
	}
	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+height {
		offset = cursor - height + 1
	}
	return clampOffset(offset, total, height)
}

func (m model) selectedRow() (row, bool) {
	if len(m.rows) == 0 {
		return row{}, false
	}
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return row{}, false
	}
	return m.rows[m.cursor], true
}

func (m *model) toggleCurrentRow(expand bool) bool {
	selected, ok := m.selectedRow()
	if !ok || !selected.Branch || strings.TrimSpace(m.filter) != "" {
		return false
	}
	if m.expanded == nil {
		m.expanded = make(map[contracts.NodeID]bool)
	}
	m.expanded[selected.Node.ID] = expand
	m.applyFilter()
	*m = m.reflow()
	return true
}

func (m *model) collapseAll() bool {
	if strings.TrimSpace(m.filter) != "" {
		return false
	}
	m.expanded = make(map[contracts.NodeID]bool)
	m.applyFilter()
	*m = m.reflow()
	return true
}

func (m *model) expandAll() bool {
	if strings.TrimSpace(m.filter) != "" {
		return false
	}
	if m.expanded == nil {
		m.expanded = make(map[contracts.NodeID]bool)
	}
	markAllExpanded(m.snapshot.Nodes, m.expanded)
	m.applyFilter()
	*m = m.reflow()
	return true
}

func (m model) hasCapability(c contracts.Capability) bool {
	if m.snapshot.Capabilities == nil {
		return false
	}
	return m.snapshot.Capabilities[c]
}

// requireCapability returns (m, true) if the capability is available.
// Otherwise it sets a consistent "not available" status and returns false.
func (m model) requireCapability(c contracts.Capability, label string) (model, bool) {
	if m.hasCapability(c) {
		return m, true
	}
	m.status = label + " is not available"
	return m, false
}

func workspaceContext(value string) string {
	workspace := strings.TrimSpace(value)
	if workspace == "" {
		return ""
	}
	return "WORKSPACE: " + workspaceLabel(workspace)
}

func workspaceLabel(workspace string) string {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return ""
	}
	if !strings.ContainsAny(workspace, `/\\`) {
		return workspace
	}

	base := filepath.Base(workspace)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	if base != "" && !strings.HasPrefix(base, ".hetki") {
		return base
	}

	parent := filepath.Base(filepath.Dir(workspace))
	if parent != "" && parent != "." && parent != string(filepath.Separator) {
		return parent
	}
	return workspace
}

func cloneIntent(intent contracts.Intent) *contracts.Intent {
	cloned := intent
	return &cloned
}
