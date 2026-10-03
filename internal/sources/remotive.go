package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

type Remotive struct{}

func (r Remotive) Fetch(url string) ([]json.RawMessage, error) {
	body, err := httpGetBody(context.Background(), url)
	if err != nil {
		return nil, err
	}

	var envelop struct {
		Jobs []json.RawMessage `json:"jobs"`
	}
	if err := json.Unmarshal(body, &envelop); err != nil {
		return nil, fmt.Errorf("failed to decode body to envelop: %w", err)
	}
	return envelop.Jobs, nil
}

func (r Remotive) ExternalID(item json.RawMessage) (string, error) {
	var v struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(item, &v); err != nil {
		return "", fmt.Errorf("remotive read id: %w", err)
	}

	return strconv.FormatInt(v.ID, 10), nil
}

func (Remotive) ToJob(raw json.RawMessage) (Job, error) {
	var v struct {
		ID              int64    `json:"id"`
		Title           string   `json:"title"`
		CompanyName     string   `json:"company_name"`
		URL             string   `json:"url"`
		Description     string   `json:"description"`
		Location        string   `json:"candidate_required_location"`
		Tags            []string `json:"tags"`
		PublicationDate string   `json:"publication_date"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return Job{}, fmt.Errorf("remotive: map job: %w", err)
	}
	var date time.Time
	if t, err := time.Parse("2006-01-02T15:04:05", v.PublicationDate); err == nil {
		date = t // Remotive date is not RFC3339; parse loosely, tolerate failure
	}
	return Job{
		ExternalID:  strconv.FormatInt(v.ID, 10),
		Title:       v.Title,
		Company:     v.CompanyName,
		URL:         v.URL,
		Description: v.Description,
		Location:    v.Location,
		Tags:        v.Tags,
		Date:        date,
	}, nil
}
