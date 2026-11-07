package config

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

			cfg, err := Load(path)

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
			if len(cfg.Sources) != 1 {
				t.Fatalf("expected 1 source; got %d", len(cfg.Sources))
			}
			if got := cfg.Sources[0]; got != tt.want {
				t.Errorf("expected %+v; got %+v", tt.want, got)
			}
		})
	}
}

func TestDigestValidate(t *testing.T) {
	valid := Digest{ModelVersion: "jev-manual-qs-v1", MinScore: 0.75, Every: 6 * time.Hour}
	tests := []struct {
		name        string
		mutate      func(d *Digest)
		errContains string
	}{
		{"valid", func(d *Digest) {}, ""},
		{"min_score 0 is allowed", func(d *Digest) { d.MinScore = 0 }, ""},
		{"empty model_version", func(d *Digest) { d.ModelVersion = "" }, "model_version"},
		{"min_score above 1", func(d *Digest) { d.MinScore = 1.5 }, "min_score"},
		{"negative min_score", func(d *Digest) { d.MinScore = -0.1 }, "min_score"},
		{"every below a minute", func(d *Digest) { d.Every = 30 * time.Second }, "every"},
		{"max_age 0 means no limit", func(d *Digest) { d.MaxAge = 0 }, ""},
		{"negative max_age", func(d *Digest) { d.MaxAge = -time.Hour }, "max_age"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := valid
			tt.mutate(&d)
			err := d.Validate()
			if tt.errContains == "" {
				if err != nil {
					t.Fatalf("expected err nil; got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.errContains) {
				t.Fatalf("expected err containing %q; got %v", tt.errContains, err)
			}
		})
	}
}
