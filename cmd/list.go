package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/MSmaili/muxie/internal/backend"
	"github.com/MSmaili/muxie/internal/manifest"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

const (
	depthRoot   = 0
	depthWindow = 1
	depthPane   = 2
)

var listCmd = &cobra.Command{
	Use:   "list [workspaces|sessions]",
	Short: "List workspaces or sessions",
	Long: `List workspace files or running tmux sessions.

Examples:
  muxie list                              # List workspace names
  muxie list workspaces --sessions        # workspace:session
  muxie list sessions --windows --format=tree  # Pretty tree view
  muxie list sessions --windows --format=json  # JSON output`,
	RunE: runList,
}

var (
	listSessions  bool
	listWindows   bool
	listPanes     bool
	listFormat    string
	listDelimiter string
	listCurrent   bool
	listMarker    string
)

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().BoolVarP(&listSessions, "sessions", "s", false, "Include sessions")
	listCmd.Flags().BoolVarP(&listWindows, "windows", "w", false, "Include windows")
	listCmd.Flags().BoolVarP(&listPanes, "panes", "p", false, "Include panes")
	listCmd.Flags().StringVarP(&listFormat, "format", "f", "flat", "Output format: flat, indent, tree, json")
	listCmd.Flags().StringVarP(&listDelimiter, "delimiter", "d", ":", "Delimiter for flat output")
	listCmd.Flags().BoolVarP(&listCurrent, "current", "c", false, "Only show current session")
	listCmd.Flags().StringVarP(&listMarker, "marker", "m", "", "Prefix for current session/window (e.g. '➤ ')")

	listCmd.ValidArgs = []string{"workspaces", "sessions"}
	listCmd.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"flat", "indent", "tree", "json"}, cobra.ShellCompDirectiveNoFileComp
	})
}

type listItem struct {
	Name    string
	Windows []listWindow
}

type listWindow struct {
	Name       string
	Panes      []int
	ActivePane int
}

type jsonSession struct {
	Name    string       `json:"name"`
	Windows []jsonWindow `json:"windows,omitempty"`
}

type jsonWindow struct {
	Name  string `json:"name"`
	Panes []int  `json:"panes,omitempty"`
}

func runList(cmd *cobra.Command, args []string) error {
	mode := "workspaces"
	if len(args) > 0 {
		mode = args[0]
	}

	if err := validateListFlags(mode); err != nil {
		return err
	}

	if mode == "sessions" {
		return listActiveSessions()
	}
	return listWorkspaceFiles()
}

func validateListFlags(mode string) error {
	validFormats := map[string]bool{"flat": true, "indent": true, "tree": true, "json": true}
	if !validFormats[listFormat] {
		return fmt.Errorf("invalid format %q\nValid formats: flat, indent, tree, json\nExample: muxie list --format=tree", listFormat)
	}
	if mode == "workspaces" {
		if listWindows && !listSessions {
			return fmt.Errorf("--windows requires --sessions\nExample: muxie list workspaces --sessions --windows")
		}
		if listCurrent {
			return fmt.Errorf("--current only works with sessions\nExample: muxie list sessions --current")
		}
		if listMarker != "" {
			return fmt.Errorf("--marker only works with sessions\nExample: muxie list sessions --marker '➤ '")
		}
	}
	if listPanes && !listWindows {
		return fmt.Errorf("--panes requires --windows\nExample: muxie list sessions --windows --panes")
	}
	return nil
}

func listWorkspaceFiles() error {
	configDir, err := manifest.GetConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	paths, err := manifest.ScanWorkspaces(configDir + "/workspaces")
	if err != nil {
		return fmt.Errorf("failed to scan workspaces: %w", err)
	}

	names := sortedKeys(paths)
	if !listSessions {
		return outputNames(names)
	}

	var (
		g       errgroup.Group
		mu      sync.Mutex
		results = make(map[string]*manifest.Workspace)
	)

	for _, wname := range names {
		name, path := wname, paths[wname]
		g.Go(func() error {
			loader := manifest.NewFileLoader(path)
			ws, err := loader.Load()
			if err != nil {
				return nil
			}
			mu.Lock()
			results[name] = ws
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	var items []listItem
	for _, name := range names {
		ws, ok := results[name]
		if !ok {
			continue
		}
		items = append(items, workspaceToItems(name, ws)...)
	}

	return outputItems(items)
}

func listActiveSessions() error {
	b, err := backend.Detect()
	if err != nil {
		return fmt.Errorf("failed to detect backend: %w\nHint: Make sure a supported multiplexer is running", err)
	}

	result, err := b.QueryState()
	if err != nil {
		return fmt.Errorf("failed to query sessions: %w", err)
	}

	sessions := result.Sessions

	if listCurrent {
		if result.Active.Session == "" {
			return fmt.Errorf("not in a session")
		}
		for _, sess := range sessions {
			if sess.Name == result.Active.Session {
				sessions = []backend.Session{sess}
				break
			}
		}
	}

	items := make([]listItem, len(sessions))
	for i, sess := range sessions {
		items[i] = sessionToItem(sess, result.Active)
	}

	return outputItems(items)
}

func workspaceToItems(name string, ws *manifest.Workspace) []listItem {
	items := make([]listItem, 0, len(ws.Sessions))

	for _, sess := range ws.Sessions {
		item := listItem{Name: name + ":" + sess.Name}
		if listWindows {
			for _, win := range sess.Windows {
				lw := listWindow{Name: win.Name}
				if listPanes {
					paneCount := max(1, len(win.Panes))
					for p := range paneCount {
						lw.Panes = append(lw.Panes, p)
					}
				}
				item.Windows = append(item.Windows, lw)
			}
		}
		items = append(items, item)
	}
	return items
}

func sessionToItem(sess backend.Session, active backend.ActiveContext) listItem {
	item := listItem{Name: applyMarker(sess.Name, sess.Name == active.Session && !listWindows)}

	if listWindows {
		for _, win := range sess.Windows {
			isActiveWindow := sess.Name == active.Session && win.Name == active.Window
			lw := listWindow{
				Name:       applyMarker(win.Name, isActiveWindow && !listPanes),
				ActivePane: -1,
			}

			if listPanes {
				if isActiveWindow {
					lw.ActivePane = active.Pane
				}
				for _, p := range win.Panes {
					lw.Panes = append(lw.Panes, p.Index)
				}
			}
			item.Windows = append(item.Windows, lw)
		}
	}
	return item
}

func applyMarker(name string, isActive bool) string {
	if listMarker != "" && isActive {
		return listMarker + name
	}
	return name
}

func outputItems(items []listItem) error {
	if listFormat == "json" {
		return outputJSON(itemsToJSON(items))
	}

	f := &formatter{format: listFormat}
	for i, item := range items {
		f.printItem(item, i == len(items)-1)
	}
	return nil
}

func itemsToJSON(items []listItem) []jsonSession {
	out := make([]jsonSession, len(items))
	for i, item := range items {
		out[i] = jsonSession{Name: item.Name}
		if len(item.Windows) > 0 {
			out[i].Windows = make([]jsonWindow, len(item.Windows))
			for j, w := range item.Windows {
				out[i].Windows[j] = jsonWindow{Name: w.Name}
				if len(w.Panes) > 0 {
					out[i].Windows[j].Panes = w.Panes
				}
			}
		}
	}
	return out
}

func outputJSON(data any) error {
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling json: %w", err)
	}
	fmt.Println(string(out))
	return nil
}

func outputNames(names []string) error {
	if listFormat == "json" {
		return outputJSON(names)
	}
	for _, n := range names {
		fmt.Println(n)
	}
	return nil
}

type formatter struct {
	format   string
	treePath []bool
}

func (f *formatter) printItem(item listItem, lastItem bool) {
	if f.format == "flat" {
		f.printFlat(item)
		return
	}
	f.printTree(item, lastItem)
}

func (f *formatter) printFlat(item listItem) {
	d := listDelimiter
	if len(item.Windows) == 0 {
		fmt.Println(item.Name)
		return
	}
	for _, win := range item.Windows {
		line := fmt.Sprintf("%s%s%s", item.Name, d, win.Name)

		if listMarker != "" && strings.HasPrefix(win.Name, listMarker) {
			cleanName := strings.TrimPrefix(win.Name, listMarker)
			line = listMarker + fmt.Sprintf("%s%s%s", item.Name, d, cleanName)
		}

		if len(win.Panes) == 0 {
			fmt.Println(line)
			continue
		}
		for _, p := range win.Panes {
			paneStr := fmt.Sprintf("%s%s%d", line, d, p)

			if listMarker != "" && win.ActivePane == p {
				cleanLine := strings.TrimPrefix(line, listMarker)
				paneStr = listMarker + fmt.Sprintf("%s%s%d", cleanLine, d, p)
			}

			fmt.Println(paneStr)
		}
	}
}

func (f *formatter) printTree(item listItem, lastItem bool) {
	if len(item.Windows) == 0 {
		f.printNode(item.Name, depthRoot, lastItem)
		return
	}

	f.printNode(item.Name, depthRoot, lastItem)
	for i, win := range item.Windows {
		lastWin := i == len(item.Windows)-1
		if len(win.Panes) == 0 {
			f.printNode(win.Name, depthWindow, lastWin)
			continue
		}
		f.printNode(win.Name, depthWindow, lastWin)
		for j, p := range win.Panes {
			f.printNode(fmt.Sprintf("%d", p), depthPane, j == len(win.Panes)-1)
		}
	}
}

func (f *formatter) printNode(name string, depth int, last bool) {
	switch f.format {
	case "indent":
		fmt.Println(strings.Repeat("  ", depth) + name)
	case "tree":
		if depth == depthRoot {
			fmt.Println(name)
			f.treePath = []bool{}
		} else {
			var prefix string
			for i := 0; i < depth-1; i++ {
				if i < len(f.treePath) && f.treePath[i] {
					prefix += "    "
				} else {
					prefix += "│   "
				}
			}
			branch := "├── "
			if last {
				branch = "└── "
			}
			fmt.Println(prefix + branch + name)
		}
		for len(f.treePath) < depth {
			f.treePath = append(f.treePath, false)
		}
		if depth > 0 {
			f.treePath[depth-1] = last
		}
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
