package tui

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/MSmaili/hetki/internal/tui/list"
)

func Run(ctx context.Context, initial list.Snapshot, dispatch DispatchFunc) (BackendTarget, error) {
	return RunWithKeyMap(ctx, initial, DefaultKeyMap(), dispatch)
}

func RunWithKeyMap(ctx context.Context, initial list.Snapshot, keys KeyMap, dispatch DispatchFunc) (BackendTarget, error) {
	m, err := newModelWithKeys(initial, dispatch, keys)
	if err != nil {
		return "", err
	}
	p := tea.NewProgram(m, tea.WithContext(ctx))
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

type uiMode string

const (
	modeBrowse  uiMode = "browse"
	modeJump    uiMode = "jump"
	modeFilter  uiMode = "filter"
	modeInput   uiMode = "input"
	modeConfirm uiMode = "confirm"
	modeMenu    uiMode = "menu"
)

type model struct {
	items       list.Model
	mode        uiMode
	status      string
	err         error
	busy        bool
	input       inputState
	confirm     confirmState
	menu        menuState
	jump        jumpState
	initialJump bool
	pending     *ActionRequest
	pendingRows []list.ItemID
	navigation  BackendTarget

	width  int
	height int

	dispatch DispatchFunc
	keys     KeyMap
	theme    theme
}

func newModel(snapshot list.Snapshot, dispatch DispatchFunc) model {
	m, err := newModelWithKeys(snapshot, dispatch, DefaultKeyMap())
	if err != nil {
		return model{err: err, dispatch: dispatch, keys: DefaultKeyMap(), theme: defaultTheme()}
	}
	return m
}

func newModelWithKeys(snapshot list.Snapshot, dispatch DispatchFunc, keys KeyMap) (model, error) {
	items, err := list.New(snapshot)
	if err != nil {
		return model{}, err
	}
	m := model{
		items:    items,
		dispatch: dispatch,
		keys:     keys,
		theme:    defaultTheme(),
		mode:     modeBrowse,
	}
	m = m.reflow()
	if len(m.items.Rows()) > 0 {
		m.initialJump = true
	}
	return m, nil
}

func (m model) reflow() model {
	m.items.Resize(m.availableListHeight())
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
	frameStyle := responsiveFrameStyle(m.theme.appBorder, lineWidth, height)
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

func (m model) selectedRow() (list.Row, bool) { return m.items.Selected() }
