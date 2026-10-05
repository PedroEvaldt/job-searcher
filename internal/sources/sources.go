package sources

import (
	"time"
)

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

// SourceConfig is one entry under "sources" in config.yaml: its name (which must match a
// row in the sources table), whether it is turned on, and the URL to fetch from.
type SourceConfig struct {
	Name    string `yaml:"name"`
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
}
