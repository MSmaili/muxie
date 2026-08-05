package manifest

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func Marshal(workspace *Workspace, path string) ([]byte, error) {
	if errs := Validate(workspace); len(errs) > 0 {
		return nil, ToError(errs)
	}
	if err := validate(workspace); err != nil {
		return nil, err
	}

	switch ext := filepath.Ext(expandPath(path)); ext {
	case ".yaml", ".yml":
		data, err := yaml.Marshal(workspace)
		if err != nil {
			return nil, fmt.Errorf("marshal yaml: %w", err)
		}
		return data, nil
	case ".json":
		data, err := json.MarshalIndent(workspace, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal json: %w", err)
		}
		return data, nil
	default:
		return nil, fmt.Errorf("unsupported format: %s (use .yaml, .yml, or .json)", ext)
	}
}
