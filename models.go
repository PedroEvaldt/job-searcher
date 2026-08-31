package main

import "time"

type Job struct {
	Slug        string    `json:"slug"`
	ID          string    `json:"id"`
	Date        time.Time `json:"date"`
	Company     string    `json:"company"`
	Position    string    `json:"position"`
	Tags        []string  `json:"tags"`
	Description string    `json:"description"`
	Location    string    `json:"location"`
	ApplyURL    string    `json:"apply_url"`
	SalaryMin   int64     `json:"salary_min"`
	SalaryMax   int64     `json:"salary_max"`
	URL         string    `json:"url"`
}

type JobID struct {
	ID string `json:"id"`
}

type JobExternals struct {
	RawJobID   int    `json:"raw_job_id"`
	SourceID   int    `json:"source_id"`
	ExternalID string `json:"external_id"`
}
