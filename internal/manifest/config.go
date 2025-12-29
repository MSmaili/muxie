package manifest

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MSmaili/tmx/internal/domain"
)

type Config struct {
	Sessions map[string]WindowList `json:"sessions"`
}
type WindowList []domain.Window

func (w *WindowList) UnmarshalJSON(data []byte) error {
	var objForm []domain.Window
	if err := json.Unmarshal(data, &objForm); err == nil {
		for i := range objForm {
			objForm[i].Index = i
		}
		*w = objForm
		return nil
	}

	var paths []string
	if err := json.Unmarshal(data, &paths); err == nil {
		windows := make([]domain.Window, len(paths))
		for i, p := range paths {
			// TODO: index can start from 1 in tmux config?
			windows[i] = domain.Window{
				Path:  p,
				Index: i,
				Name:  inferNameFromPath(p),
			}
		}
		*w = windows
		return nil
	}

	return fmt.Errorf("invalid window list format: %s", string(data))
}

func inferNameFromPath(p string) string {
	if p == "" {
		return ""
	}
	parts := strings.Split(p, "/")
	return parts[len(parts)-1]
}
