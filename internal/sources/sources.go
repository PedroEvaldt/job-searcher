package sources

import (
	"context"
	"fmt"
	"io"
	"net/http"
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

func httpGetBody(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed do build request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch: %s; status: %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
