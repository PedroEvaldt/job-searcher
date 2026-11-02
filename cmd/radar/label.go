package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

var (
	brTag      = regexp.MustCompile(`(?i)<br\s*/?>`)
	blockClose = regexp.MustCompile(`(?i)</(p|div|li|ul|ol|h[1-6])>`)
	anyTag     = regexp.MustCompile(`<[^>]+>`)
	blankRuns  = regexp.MustCompile(`\n{3,}`)
)

// cleanDescription makes a job description readable on screen while labelling:
// it turns line/block tags into newlines, drops every other tag, decodes HTML
// entities, and collapses blank-line runs. It is a DISPLAY helper only — the
// description stored in the database is never touched.
func cleanDescription(desc string) string {
	desc = brTag.ReplaceAllString(desc, "\n")
	desc = blockClose.ReplaceAllString(desc, "\n")
	desc = anyTag.ReplaceAllString(desc, "")
	// Unescape AFTER stripping, so literal "&lt;b&gt;" in the text is not turned
	// into a real tag and then removed.
	desc = html.UnescapeString(desc)
	desc = blankRuns.ReplaceAllString(desc, "\n")
	return strings.TrimSpace(desc)
}

// labelQueue is the file sample_label_round.py writes: the job ids to label, in order.
type labelQueue struct {
	JobIDs []int64 `json:"job_ids"`
}

func loadLabelQueue(path string) ([]int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read label queue %s: %w", path, err)
	}
	var q labelQueue
	if err := json.Unmarshal(data, &q); err != nil {
		return nil, fmt.Errorf("parse label queue %s: %w", path, err)
	}
	if len(q.JobIDs) == 0 {
		return nil, fmt.Errorf("label queue %s has no job_ids", path)
	}
	return q.JobIDs, nil
}

// label walks the unlabelled live jobs one at a time. With a queue it only shows the
// queue's jobs, in the queue's order, skipping the ones already labelled — so a
// session can stop anywhere and the next one resumes. Running out of jobs or
// hitting EOF (Ctrl+D) is a normal way to stop, not an error.
func label(ctx context.Context, conn *pgx.Conn, limit int, queue []int64) error {
	reader := bufio.NewReader(os.Stdin)
	var (
		jobID       int
		remaining   int
		title       string
		company     string
		description string
	)
	counter := 0
	for {
		if counter >= limit {
			return nil
		}
		var row pgx.Row
		if queue == nil {
			row = conn.QueryRow(ctx, "SELECT j.id, j.title, j.company, j.description FROM jobs j LEFT JOIN labels l ON j.id = l.job_id WHERE l.labeled_at IS NULL AND j.duplicate_of IS NULL ORDER BY j.id LIMIT 1;")
		} else {
			// array_position keeps the queue's order; no duplicate_of filter, since the
			// queue was drawn from live jobs and a test job must stay labelable.
			row = conn.QueryRow(ctx, `
				SELECT j.id, j.title, j.company, j.description,
				       (SELECT count(*) FROM unnest($1::bigint[]) q(id)
				        WHERE NOT EXISTS (SELECT 1 FROM labels WHERE job_id = q.id)) AS remaining
				FROM jobs j LEFT JOIN labels l ON j.id = l.job_id
				WHERE j.id = ANY($1) AND l.job_id IS NULL
				ORDER BY array_position($1, j.id)
				LIMIT 1;`, queue)
		}
		var err error
		if queue == nil {
			err = row.Scan(&jobID, &title, &company, &description)
		} else {
			err = row.Scan(&jobID, &title, &company, &description, &remaining)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			fmt.Println("nothing left to label")
			return nil
		}
		if err != nil {
			return fmt.Errorf("query next job to label: %w", err)
		}
		clearScreen()
		if queue != nil {
			fmt.Printf("[%d of %d left in the queue]\n\n", remaining, len(queue))
		}
		fmt.Printf("Title: %s\nCompany: %s\n\n%s\n", title, company, cleanDescription(description))

		fmt.Print("Would apply (y/n): ")
		var (
			wouldApply bool = false
			hiresLatam *bool
			seniority  *string
			note       *string
		)
	outer:
		for {
			wouldApplyStr, err := reader.ReadString('\n')
			if errors.Is(err, io.EOF) {
				fmt.Println("shutting down")
				return nil
			}
			if err != nil {
				return fmt.Errorf("read user input: %w", err)
			}
			wouldApplyStr = strings.TrimSpace(wouldApplyStr)
			switch {
			case wouldApplyStr == "y":
				wouldApply = true
				break outer
			case wouldApplyStr == "n":
				wouldApply = false
				break outer
			default:
				fmt.Printf("answer must be y or n!\n")
			}
		}
		fmt.Printf("Hires Latam (y|n): ")
		hiresLatamStr, err := reader.ReadString('\n')
		if errors.Is(err, io.EOF) {
			fmt.Println("shutting down")
			return nil
		}
		if err != nil {
			return fmt.Errorf("read user input: %w", err)
		}
		if strings.TrimSpace(hiresLatamStr) == "y" {
			v := true
			hiresLatam = &v
		}
		fmt.Printf("Seniority (junior|mid-level|senior): ")
		seniorityStr, err := reader.ReadString('\n')
		if errors.Is(err, io.EOF) {
			fmt.Println("shutting down")
			return nil
		}
		if err != nil {
			return fmt.Errorf("read user input: %w", err)
		}
		if s := strings.TrimSpace(seniorityStr); s != "" {
			seniority = &s
		}
		fmt.Printf("Note: ")
		noteStr, err := reader.ReadString('\n')
		if errors.Is(err, io.EOF) {
			fmt.Println("shutting down")
			return nil
		}
		if err != nil {
			return fmt.Errorf("read user input: %w", err)
		}
		if s := strings.TrimSpace(noteStr); s != "" {
			note = &s
		}
		_, err = conn.Exec(ctx, "INSERT INTO labels (job_id, would_apply, hires_latam, seniority, note, labeled_at) VALUES ($1, $2, $3, $4, $5, now());", jobID, wouldApply, hiresLatam, seniority, note)
		if err != nil {
			return fmt.Errorf("insert label for job %d: %w", jobID, err)
		}
		counter++
	}
}

func clearScreen() {
	fmt.Print("\033[H\033[2J") // cursor to home, then clear the whole screen
}
