package sources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/PedroEvaldt/job-searcher/internal/client"
)

// RemoteOK reads the Remote OK feed: one request, a JSON array whose first element
// is a legal notice, not a job.
type RemoteOK struct {
	C *client.Client
}

func (r RemoteOK) Fetch(ctx context.Context, url string) ([]json.RawMessage, error) {
	body, err := r.C.HTTPGetBody(ctx, url)
	if err != nil {
		return nil, err
	}
	var items []json.RawMessage
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("remoteok: decode body: %w", err)
	}
	// The legal notice comes first even when there are no jobs, so an empty array
	// means the feed changed, not that it ran out of jobs.
	if len(items) == 0 {
		return nil, errors.New("remoteok: empty feed, the legal notice is missing")
	}
	return items[1:], nil
}

func (r RemoteOK) ExternalID(item json.RawMessage) (string, error) {
	var v struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(item, &v); err != nil {
		return "", fmt.Errorf("remoteok: read id: %w", err)
	}
	return v.ID, nil
}

func (RemoteOK) ToJob(raw json.RawMessage) (Job, error) {
	var v struct {
		ID          string    `json:"id"`
		Position    string    `json:"position"`
		Company     string    `json:"company"`
		URL         string    `json:"url"`
		Description string    `json:"description"`
		SalaryMin   float64   `json:"salary_min"`
		SalaryMax   float64   `json:"salary_max"`
		Location    string    `json:"location"`
		Tags        []string  `json:"tags"`
		Date        time.Time `json:"date"` // Remote OK sends RFC3339
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return Job{}, fmt.Errorf("remoteok: map job: %w", err)
	}
	return Job{
		ExternalID:  v.ID,
		Title:       v.Position,
		Company:     v.Company,
		URL:         v.URL,
		Description: v.Description,
		SalaryMin:   nonZero(v.SalaryMin),
		SalaryMax:   nonZero(v.SalaryMax),
		Location:    v.Location,
		Tags:        v.Tags,
		Date:        v.Date,
	}, nil
}
