package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/PedroEvaldt/job-searcher/internal/client"
)

// Himalayas pages through the Himalayas feed by cursor, up to MaxPages pages.
type Himalayas struct {
	C        *client.Client
	MaxPages int
}

func (r Himalayas) Fetch(ctx context.Context, rawURL string) ([]json.RawMessage, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %w", err)
	}
	cursor := ""
	var items []json.RawMessage
	for i := range r.MaxPages {
		if cursor != "" {
			q := u.Query()
			q.Set("cursor", cursor)
			u.RawQuery = q.Encode()
		}
		body, err := r.C.HTTPGetBody(ctx, u.String())
		if err != nil {
			return nil, fmt.Errorf("fetch page %d: %w", i+1, err)
		}
		var envelope struct {
			Jobs       []json.RawMessage `json:"jobs"`
			NextCursor string            `json:"nextCursor"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			return nil, fmt.Errorf("decode page %d: %w", i+1, err)
		}
		// Append before checking the cursor: the last page carries jobs too.
		items = append(items, envelope.Jobs...)
		if envelope.NextCursor == "" {
			break
		}
		cursor = envelope.NextCursor
	}
	return items, nil
}

func (r Himalayas) ExternalID(item json.RawMessage) (string, error) {
	var v struct {
		GUID string `json:"guid"`
	}
	if err := json.Unmarshal(item, &v); err != nil {
		return "", fmt.Errorf("himalayas: read id: %w", err)
	}
	return v.GUID, nil
}

func (Himalayas) ToJob(raw json.RawMessage) (Job, error) {
	var v struct {
		GUID        string   `json:"guid"`
		URL         string   `json:"applicationLink"`
		Title       string   `json:"title"`
		CompanyName string   `json:"companyName"`
		Description string   `json:"description"`
		Location    []string `json:"locationRestrictions"`
		Categories  []string `json:"categories"`
		PubDate     int64    `json:"pubDate"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return Job{}, fmt.Errorf("himalayas: map job: %w", err)
	}
	date := time.Unix(v.PubDate, 0).UTC()
	location := strings.Join(v.Location, ",")
	return Job{
		ExternalID:  v.GUID,
		Title:       v.Title,
		Company:     v.CompanyName,
		URL:         v.URL,
		Description: v.Description,
		Location:    location,
		Tags:        v.Categories,
		Date:        date,
	}, nil
}
