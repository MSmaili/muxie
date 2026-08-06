package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	applist "github.com/MSmaili/hetki/internal/app/list"
	"github.com/MSmaili/hetki/internal/terminal"
)

const (
	depthRoot   = 0
	depthWindow = 1
	depthPane   = 2
)

type jsonSession struct {
	Name    string       `json:"name"`
	Windows []jsonWindow `json:"windows,omitempty"`
}

type jsonWindow struct {
	Name  string `json:"name"`
	Panes []int  `json:"panes,omitempty"`
}

func outputItems(items []applist.Item) error {
	if listFormat == "json" {
		return outputJSON(itemsToJSON(items))
	}

	f := &formatter{format: listFormat}
	for i, item := range items {
		f.printItem(item, i == len(items)-1)
	}
	return nil
}

func itemsToJSON(items []applist.Item) []jsonSession {
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
		fmt.Println(terminal.Sanitize(n))
	}
	return nil
}

type formatter struct {
	format   string
	treePath []bool
}

func (f *formatter) printItem(item applist.Item, lastItem bool) {
	if f.format == "flat" {
		f.printFlat(item)
		return
	}
	f.printTree(item, lastItem)
}

func (f *formatter) printFlat(item applist.Item) {
	d := listDelimiter
	if len(item.Windows) == 0 {
		fmt.Println(terminal.Sanitize(item.Name))
		return
	}
	for _, win := range item.Windows {
		line := fmt.Sprintf("%s%s%s", item.Name, d, win.Name)

		if listMarker != "" && strings.HasPrefix(win.Name, listMarker) {
			cleanName := strings.TrimPrefix(win.Name, listMarker)
			line = listMarker + fmt.Sprintf("%s%s%s", item.Name, d, cleanName)
		}

		if len(win.Panes) == 0 {
			fmt.Println(terminal.Sanitize(line))
			continue
		}
		for _, p := range win.Panes {
			paneStr := fmt.Sprintf("%s%s%d", line, d, p)

			if listMarker != "" && win.ActivePane == p {
				cleanLine := strings.TrimPrefix(line, listMarker)
				paneStr = listMarker + fmt.Sprintf("%s%s%d", cleanLine, d, p)
			}

			fmt.Println(terminal.Sanitize(paneStr))
		}
	}
}

func (f *formatter) printTree(item applist.Item, lastItem bool) {
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
		fmt.Println(strings.Repeat("  ", depth) + terminal.Sanitize(name))
	case "tree":
		if depth == depthRoot {
			fmt.Println(terminal.Sanitize(name))
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
			fmt.Println(prefix + branch + terminal.Sanitize(name))
		}
		for len(f.treePath) < depth {
			f.treePath = append(f.treePath, false)
		}
		if depth > 0 {
			f.treePath[depth-1] = last
		}
	}
}
