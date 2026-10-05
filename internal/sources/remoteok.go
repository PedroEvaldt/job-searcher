package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PedroEvaldt/job-searcher/internal/client"
)

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
		return nil, fmt.Errorf("failed to decode request body: %w", err)
	}
	return items[1:], nil
}

func (r RemoteOK) ExternalID(item json.RawMessage) (string, error) {
	var v struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(item, &v); err != nil {
		return "", fmt.Errorf("remoteok read id: %w", err)
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
		Location:    v.Location,
		Tags:        v.Tags,
		Date:        v.Date,
	}, nil
}
