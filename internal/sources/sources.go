package sources

import (
	"time"
)

// Job is the common model every source maps its items to.
type Job struct {
	ExternalID  string
	Title       string
	Company     string
	URL         string
	Description string
	Location    string
	Tags        []string
	Date        time.Time
}

// SourceConfig is one entry under "sources" in config.yaml. Name must match a row in
// the sources table.
type SourceConfig struct {
	Name        string        `yaml:"name"`
	Enabled     bool          `yaml:"enabled"`
	URL         string        `yaml:"url"`
	Retries     int           `yaml:"retries"`      // total attempts per request, the first one included
	Timeout     time.Duration `yaml:"timeout"`      // budget for all of the source's work in one run
	InitialWait time.Duration `yaml:"initial_wait"` // backoff before the first retry; doubles after each one
	Interval    time.Duration `yaml:"interval"`     // minimum gap between requests (rate limit)
}
