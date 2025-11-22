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
		test  func(t *testing.T, cfg *Config, err error)
	}{
		{
			name: "simple lazy paths should work",
			input: j{
				"sessions": j{
					"work": []any{"~/project", "~/api"},
				},
			},
			test: func(t *testing.T, cfg *Config, err error) {
				assert.Nil(t, err)
				windows := cfg.Sessions["work"]
				assert.Equal(t, len(windows), 2)
				assert.Equal(t, windows[0].Name, "project")
				assert.Equal(t, windows[0].Name, "project")
			},
		},
		{
			name: "advanced windows",
			input: j{
				"sessions": j{
					"work": []any{
						j{"name": "api", "path": "~/api"},
						j{"path": "~/project"},
					},
				},
			},
			test: func(t *testing.T, cfg *Config, err error) {
				assert.Nil(t, err)
				w := cfg.Sessions["work"]

				assert.Equal(t, w[0].Name, "api")
				assert.Equal(t, w[1].Name, "project")
			},
		},
		{
			name: "error: missing path",
			input: j{
				"sessions": j{
					"work": []any{
						j{"name": "x"},
					},
				},
			},
			test: func(t *testing.T, cfg *Config, err error) {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
			},
		},
		{
			name:  "error: missing sessions",
			input: j{},
			test: func(t *testing.T, cfg *Config, err error) {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
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
		test  func(t *testing.T, cfg *Config, err error)
	}{
		{
			name: "valid simple config",
			input: j{
				"sessions": j{
					"demo": []any{"~/foo"},
				},
			},
			test: func(t *testing.T, cfg *Config, err error) {
				assert.Nil(t, err)
				assert.Equal(t, "foo", cfg.Sessions["demo"][0].Name)
			},
		},
		{
			name:  "invalid json",
			input: j{"sessions": 123},
			test: func(t *testing.T, cfg *Config, err error) {
				if err == nil {
					t.Fatalf("expected error")
				}
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
