package main

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFormatDigest(t *testing.T) {
	job := func(id int64, title string) digestJob {
		return digestJob{id: id, title: title, company: "Acme", url: "https://example.com/" + title, score: 0.8}
	}
	three := []digestJob{job(1, "Go Engineer"), job(2, "Python Developer"), job(3, "Engenheiro de Dados")}
	oneEntry := utf8.RuneCountInString(digestHeader) + utf8.RuneCountInString(
		"**Go Engineer** — Acme\nScore: 80%\n<https://example.com/Go Engineer>\n\n")

	tests := []struct {
		name         string
		jobs         []digestJob
		limit        int
		wantIncluded int
	}{
		{"everything fits", three, discordMaxContent, 3},
		// Exactly one entry's worth of room: the second job must be left out whole,
		// never cut in half.
		{"stops at the limit", three, oneEntry, 1},
		{"one rune short of the first entry", three, oneEntry - 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, included := formatDigest(tt.jobs, tt.limit)
			if got := len(included); got != tt.wantIncluded {
				t.Fatalf("included %d jobs, want %d", got, tt.wantIncluded)
			}
			if n := utf8.RuneCountInString(msg); n > tt.limit {
				t.Errorf("message has %d runes, limit is %d", n, tt.limit)
			}
			for _, j := range included {
				if !strings.Contains(msg, j.title) {
					t.Errorf("included job %d is missing from the message", j.id)
				}
			}
		})
	}
}

func TestFormatDigestLongTitle(t *testing.T) {
	// 2000 runes but 4000 bytes: a byte-based cut would split a rune in half.
	title := strings.Repeat("ç", 2000)
	msg, included := formatDigest([]digestJob{{id: 1, title: title, company: "Acme", url: "https://example.com"}}, discordMaxContent)

	if len(included) != 1 {
		t.Fatalf("a long title must be truncated, not drop the job: included %d", len(included))
	}
	if !utf8.ValidString(msg) {
		t.Error("message is not valid UTF-8")
	}
	if n := utf8.RuneCountInString(msg); n > discordMaxContent {
		t.Errorf("message has %d runes, limit is %d", n, discordMaxContent)
	}
}

func TestSendNotificationHidesWebhookURL(t *testing.T) {
	// Port 1 refuses the connection right away, so this needs no network.
	err := sendNotification(context.Background(), "hi", "http://127.0.0.1:1/api/webhooks/1/SECRET-TOKEN")
	if err == nil {
		t.Fatal("expected an error from a closed port")
	}
	if strings.Contains(err.Error(), "SECRET-TOKEN") {
		t.Errorf("error leaks the webhook token: %v", err)
	}
}
