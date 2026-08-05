package core

import (
	"context"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/MSmaili/hetki/internal/tui/contracts"
)

type DispatchFunc func(context.Context, contracts.Intent) (contracts.ActionResult, error)

func Run(ctx context.Context, initial contracts.Snapshot, dispatch DispatchFunc) error {
	p := tea.NewProgram(newModel(initial, dispatch), tea.WithContext(ctx))
	_, err := p.Run()
	return err
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
	Target       string
	Payload      map[string]string
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
	snapshot contracts.Snapshot
	rows     []row
	cursor   int
	offset   int
	listH    int
	mode     uiMode
	filter   string
	status   string
	err      error
	busy     bool
	input    inputState
	confirm  confirmState
	expanded map[string]bool
	pending  *contracts.Intent
	helpOpen bool

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

func (m model) availableListHeight() int {
	height := m.height
	if height <= 0 {
		height = 24
	}
	list := height - m.reservedLines()
	if list < 3 {
		return 3
	}
	return list
}

func (m model) reservedLines() int {
	reserved := 8
	if m.mode == modeInput || m.mode == modeConfirm {
		reserved += 5
	}
	return reserved
}

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
		m.expanded = make(map[string]bool)
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
	m.expanded = make(map[string]bool)
	m.applyFilter()
	*m = m.reflow()
	return true
}

func (m *model) expandAll() bool {
	if strings.TrimSpace(m.filter) != "" {
		return false
	}
	if m.expanded == nil {
		m.expanded = make(map[string]bool)
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

func workspaceContext(ctx map[string]string) string {
	if ctx == nil {
		return ""
	}
	workspace := strings.TrimSpace(ctx["workspace"])
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

func truncateWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	visualW := lipgloss.Width(s)
	if visualW <= width {
		return s
	}
	r := []rune(s)
	if width <= 3 {
		return string(r[:width])
	}
	lipglossWidth := 0
	i := 0
	for i < len(r) {
		segW := lipgloss.Width(string(r[i]))
		if lipglossWidth+segW > width-3 {
			break
		}
		lipglossWidth += segW
		i++
	}
	return string(r[:i]) + "..."
}

func (m model) modeLabel() string {
	switch m.mode {
	case modeFilter:
		return "filter"
	case modeInput:
		return "input"
	case modeConfirm:
		return "confirm"
	default:
		return "browse"
	}
}

func cloneIntent(intent contracts.Intent) *contracts.Intent {
	cloned := contracts.Intent{
		Type:    intent.Type,
		Target:  intent.Target,
		Payload: clonePayloadMap(intent.Payload),
	}
	return &cloned
}

func clonePayloadMap(payload map[string]string) map[string]string {
	if payload == nil {
		return nil
	}
	cloned := make(map[string]string, len(payload))
	for k, v := range payload {
		cloned[k] = v
	}
	return cloned
}
