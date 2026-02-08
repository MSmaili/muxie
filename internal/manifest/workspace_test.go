package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

type j map[string]any

func TestLoadFromMemory(t *testing.T) {
	tests := []struct {
		name  string
		input j
		test  func(t *testing.T, cfg *Workspace, err error)
	}{
		{
			name: "simple session with windows",
			input: j{
				"sessions": []any{
					j{
						"name": "work",
						"windows": []any{
							j{"name": "api", "path": "~/api"},
							j{"name": "project", "path": "~/project"},
						},
					},
				},
			},
			test: func(t *testing.T, cfg *Workspace, err error) {
				assert.Nil(t, err)
				assert.Equal(t, 1, len(cfg.Sessions))
				assert.Equal(t, "work", cfg.Sessions[0].Name)
				assert.Equal(t, 2, len(cfg.Sessions[0].Windows))
				assert.Equal(t, "api", cfg.Sessions[0].Windows[0].Name)
			},
		},
		{
			name: "session with root inherits to windows",
			input: j{
				"sessions": []any{
					j{
						"name": "work",
						"root": "~/projects",
						"windows": []any{
							j{"name": "editor"},
						},
					},
				},
			},
			test: func(t *testing.T, cfg *Workspace, err error) {
				assert.Nil(t, err)
				assert.Contains(t, cfg.Sessions[0].Windows[0].Path, "projects")
			},
		},
		{
			name: "error: missing path and no root",
			input: j{
				"sessions": []any{
					j{
						"name": "work",
						"windows": []any{
							j{"name": "x"},
						},
					},
				},
			},
			test: func(t *testing.T, cfg *Workspace, err error) {
				assert.NotNil(t, err)
			},
		},
		{
			name:  "error: missing sessions",
			input: j{},
			test: func(t *testing.T, cfg *Workspace, err error) {
				assert.NotNil(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, _ := json.Marshal(tt.input)
			cfg, err := loadFromMemory([]byte(d))
			tt.test(t, cfg, err)
		})
	}
}

func TestFileLoader(t *testing.T) {
	tests := []struct {
		name  string
		input j
		test  func(t *testing.T, cfg *Workspace, err error)
	}{
		{
			name: "valid simple config",
			input: j{
				"sessions": []any{
					j{
						"name": "demo",
						"windows": []any{
							j{"path": "~/foo"},
						},
					},
				},
			},
			test: func(t *testing.T, cfg *Workspace, err error) {
				assert.Nil(t, err)
				assert.Equal(t, "foo", cfg.Sessions[0].Windows[0].Name)
			},
		},
		{
			name:  "invalid json",
			input: j{"sessions": 123},
			test: func(t *testing.T, cfg *Workspace, err error) {
				assert.NotNil(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.json")
			b, _ := json.Marshal(tt.input)

			err := os.WriteFile(path, []byte(b), 0644)
			if err != nil {
				t.Fatalf("Error on writing on temp file")
			}

			loader := NewFileLoader(path)
			cfg, err := loader.Load()
			tt.test(t, cfg, err)
		})
	}
}
