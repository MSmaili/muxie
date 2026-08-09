package manifest

import (
	"fmt"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func Marshal(workspace *Workspace, path string) ([]byte, error) {
	if _, err := prepare(workspace); err != nil {
		return nil, err
	}

	ext := filepath.Ext(expandPath(path))
	if ext != ".yaml" && ext != ".yml" {
		return nil, fmt.Errorf("unsupported format: %s (use .yaml or .yml)", ext)
	}
	data, err := yaml.Marshal(workspace)
	if err != nil {
		return nil, fmt.Errorf("marshal yaml: %w", err)
	}
	if len(data) > MaxManifestBytes {
		return nil, fmt.Errorf("marshaled manifest exceeds %d bytes", MaxManifestBytes)
	}
	return data, nil
}
