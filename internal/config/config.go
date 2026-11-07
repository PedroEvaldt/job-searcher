// Package config parses config.yaml, the one versioned settings file shared by the
// collector and radar. Secrets never live here: they come from environment variables.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/PedroEvaldt/job-searcher/internal/sources"
	"go.yaml.in/yaml/v3"
)

// Path is where both binaries look for the file, relative to their working directory
// (/app in the images, the repository root when run locally).
const Path = "./config.yaml"

// Digest is the digest: section, read by radar.
type Digest struct {
	// ModelVersion picks which scores.model_version the digest ranks by.
	ModelVersion string `yaml:"model_version"`
	// MinScore is the lowest score a job may have to be sent, on the model's own scale.
	MinScore float64 `yaml:"min_score"`
	// Every is how often digest-loop sends a digest.
	Every time.Duration `yaml:"every"`
	// MaxAge leaves out jobs posted longer ago than this (and jobs with no posting
	// date). Zero sends jobs of any age.
	MaxAge time.Duration `yaml:"max_age"`
}

// Config is the parsed config.yaml.
type Config struct {
	CollectEvery time.Duration          `yaml:"collect_every"`
	Sources      []sources.SourceConfig `yaml:"sources"`
	Digest       Digest                 `yaml:"digest"`
}

// Load reads the YAML file at path, rejects unknown keys, and validates every enabled
// source. A missing key decodes to its zero value, so zero or negative timings and
// retries are rejected here instead of failing obscurely at run time. The digest:
// section is checked separately, by Digest.Validate, since only radar uses it.
func Load(path string) (*Config, error) {
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
		return nil, errors.New("collect_every must be greater than 0")
	}

	for _, c := range cfg.Sources {
		if !c.Enabled {
			continue
		}
		if c.Name == "" {
			return nil, errors.New("config name must not be empty")
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

// Validate rejects a digest: section that could never send anything. It is an error,
// not a warning: with min_score wrong the digest would send nothing forever while the
// scheduler sees a clean exit.
func (d Digest) Validate() error {
	if d.ModelVersion == "" {
		return errors.New("digest.model_version must not be empty")
	}
	if d.MinScore < 0 || d.MinScore > 1 {
		return fmt.Errorf("digest.min_score is %v, expected 0-1", d.MinScore)
	}
	if d.Every < time.Minute {
		return fmt.Errorf("digest.every is %v, expected at least 1m", d.Every)
	}
	if d.MaxAge < 0 {
		return fmt.Errorf("digest.max_age is %v, expected 0 (no limit) or more", d.MaxAge)
	}
	return nil
}
