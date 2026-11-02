package main

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/PedroEvaldt/job-searcher/internal/sources"
	"go.yaml.in/yaml/v3"
)

type Digest struct {
	ModelVersion string        `yaml:"model_version"`
	MinScore     float64       `yaml:"min_score"`
	Every        time.Duration `yaml:"every"`
}

// Config is the parsed config.yaml: the list of sources the collector may pull from.
type Config struct {
	CollectEvery  time.Duration          `yaml:"collect_every"`
	SourcesConfig []sources.SourceConfig `yaml:"sources"`
	Digest        Digest                 `yaml:"digest"`
}

// loadConfig reads the YAML file at path, rejects unknown keys, and validates every
// enabled source. A missing key decodes to its zero value, so zero or negative
// timings and retries are rejected here instead of failing obscurely at run time.
func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	if cfg.CollectEvery <= 0 {
		return nil, fmt.Errorf("collect_every must be greater than 0")
	}

	for _, c := range cfg.SourcesConfig {
		if !c.Enabled {
			continue
		}
		if c.Name == "" {
			return nil, fmt.Errorf("config name must not be empty")
		}
		if c.URL == "" {
			return nil, fmt.Errorf("config %s URL must not be empty", c.Name)
		}
		if c.Timeout <= 0 {
			return nil, fmt.Errorf("config %s timeout must be greater than 0", c.Name)
		}
		if c.Retries <= 0 {
			return nil, fmt.Errorf("config %s retries must be greater than 0", c.Name)
		}
		if c.InitialWait <= 0 {
			return nil, fmt.Errorf("config %s initial_wait must be greater than 0", c.Name)
		}
		if c.Interval <= 0 {
			return nil, fmt.Errorf("config %s interval must be greater than 0", c.Name)
		}
	}
	return &cfg, nil
}
