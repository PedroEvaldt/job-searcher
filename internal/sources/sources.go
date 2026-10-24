package sources

import (
	"time"
)

// Job is the common model every source maps its items to.
type Job struct {
	ExternalID   string
	Title        string
	Company      string
	URL          string
	Description  string
	Location     string
	SalaryPeriod *string
	Currency     *string
	SalaryMin    *float64
	SalaryMax    *float64
	Tags         []string
	Date         time.Time
}

// nonZero returns a pointer to n, or nil when n is 0 — the "not informed" sentinel
// sources use for salary (eda.md defect 5). A nil pointer is stored as NULL.
func nonZero(n float64) *float64 {
	if n == 0 {
		return nil
	}
	return &n
}

// nonEmpty returns a pointer to s, or nil when s is empty, so a missing currency or
// period is stored as NULL rather than "".
func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
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
