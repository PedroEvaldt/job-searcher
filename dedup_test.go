package main

import (
	"testing"

	"github.com/PedroEvaldt/job-searcher/internal/sources"
)

// TestIsDuplicate runs the dedupe rule against real cases from the week of data on
// the Pi (1.5.1). Descriptions are short stand-ins for the real ones.
func TestIsDuplicate(t *testing.T) {
	// Source ids only need to be equal or different; the names are for reading.
	const (
		himalayas = 1
		remoteok  = 2
		remotive  = 3
	)

	tests := []struct {
		name    string
		job1    sources.Job
		source1 int
		job2    sources.Job
		source2 int
		want    bool
	}{
		{
			"clones in one source",
			sources.Job{Company: "CapsLock", Title: "Social Media Video Editor & Designer", Description: "Edit short videos."},
			himalayas,
			sources.Job{Company: "CapsLock", Title: "Social Media Video Editor & Designer", Description: "Edit short videos."},
			himalayas,
			true,
		},
		{
			"same source, hybrid and remote",
			sources.Job{Company: "Bjak", Title: "Engineering Manager", Description: "Hybrid role, 3 days in the office."},
			himalayas,
			sources.Job{Company: "Bjak", Title: "Engineering Manager", Description: "Remote role, we hire globally."},
			himalayas,
			false,
		},
		{
			"punctuation differs across sources",
			sources.Job{Company: "Huzzle", Title: "Junior Sales Development Representative - IT Industry", Description: "<p>Sell IT services.</p>"},
			himalayas,
			sources.Job{Company: "Huzzle", Title: "Junior Sales Development Representative IT Industry", Description: "Sell IT services."},
			remoteok,
			true,
		},
		{
			"hyphen differs across sources",
			sources.Job{Company: "Lemon.io", Title: "Senior React Full-stack Developer", Description: "<p>Build React apps.</p>"},
			remotive,
			sources.Job{Company: "Lemon.io", Title: "Senior React Full stack Developer", Description: "Build React apps."},
			remoteok,
			true,
		},
		{
			"different titles, same description",
			sources.Job{Company: "CapsLock", Title: "Product Manager", Description: "Join CapsLock."},
			himalayas,
			sources.Job{Company: "CapsLock", Title: "Senior CRO Strategist", Description: "Join CapsLock."},
			himalayas,
			false,
		},
		{
			"title with no a-z or 0-9",
			sources.Job{Company: "Gate", Title: "内容风控专家", Description: "Content risk."},
			himalayas,
			sources.Job{Company: "Gate", Title: "测试负责人", Description: "Test lead."},
			remoteok,
			false,
		},
		{
			"company in capitals",
			sources.Job{Company: "MWDN", Title: "Security Researcher", Description: "Find security bugs."},
			himalayas,
			sources.Job{Company: "MWDN", Title: "Security Researcher", Description: "Find security bugs."},
			himalayas,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDuplicate(tt.job1, tt.job2, tt.source1, tt.source2)
			if got != tt.want {
				t.Errorf("expected %v; got %v", tt.want, got)
			}
		})
	}
}
