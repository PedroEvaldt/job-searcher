package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PedroEvaldt/job-searcher/internal/sources"
)

func TestLoad(t *testing.T) {
	// YAML forbids tabs for indentation, so the documents below start at column 0.
	tests := []struct {
		name        string
		yaml        string
		want        sources.SourceConfig
		wantErr     bool
		errContains []string
	}{
		{
			name: "valid",
			yaml: `
collect_every: 6h
sources:
  - name: remotive
    enabled: true
    url: https://remotive.com/api/remote-jobs
    timeout: 30s
    retries: 3
    initial_wait: 5s
    interval: 1s
`,
			want: sources.SourceConfig{
				Name:        "remotive",
				Enabled:     true,
				URL:         "https://remotive.com/api/remote-jobs",
				Timeout:     30 * time.Second,
				Retries:     3,
				InitialWait: 5 * time.Second,
				Interval:    time.Second,
			},
		},
		{
			name: "enabled source without retries",
			yaml: `
collect_every: 6h
sources:
  - name: remotive
    enabled: true
    url: https://remotive.com/api/remote-jobs
    timeout: 30s
    initial_wait: 5s
    interval: 1s
`,
			wantErr:     true,
			errContains: []string{"remotive", "retries"},
		},
		{
			name: "enabled source without interval",
			yaml: `
collect_every: 6h
sources:
  - name: remotive
    enabled: true
    url: https://remotive.com/api/remote-jobs
    timeout: 30s
    retries: 3
    initial_wait: 5s
`,
			wantErr:     true,
			errContains: []string{"remotive", "interval"},
		},
		{
			name: "disabled source without retry fields",
			yaml: `
collect_every: 6h
sources:
  - name: remotive
    enabled: false
    url: https://remotive.com/api/remote-jobs
`,
			want: sources.SourceConfig{
				Name: "remotive",
				URL:  "https://remotive.com/api/remote-jobs",
			},
		},
		{
			name: "without collect_every",
			yaml: `
sources:
  - name: remotive
    enabled: false
    url: https://remotive.com/api/remote-jobs
    timeout: 30s
    retries: 3
    initial_wait: 5s
`,
			wantErr:     true,
			errContains: []string{"collect_every", "greater"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0o644); err != nil {
				t.Fatalf("failed to write config: %v", err)
			}

			cfg, err := loadConfig(path)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected err; got nil")
				}
				for _, s := range tt.errContains {
					if !strings.Contains(err.Error(), s) {
						t.Errorf("expected err to contain %q; got %v", s, err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("expected err nil; got %v", err)
			}
			if len(cfg.SourcesConfig) != 1 {
				t.Fatalf("expected 1 source; got %d", len(cfg.SourcesConfig))
			}
			if got := cfg.SourcesConfig[0]; got != tt.want {
				t.Errorf("expected %+v; got %+v", tt.want, got)
			}
		})
	}
}
