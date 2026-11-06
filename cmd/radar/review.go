package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
)

// jobReview is one answer given during a review session.
type jobReview struct {
	jobID int
	keep  bool
}

// review walks every job labelled would_apply = true and asks whether to keep it;
// "n" flips the label to false, "y" leaves it alone. Answers are collected while the
// rows are read and written only afterwards, since the connection cannot run an
// UPDATE while a query is still open on it. Ctrl+D ends the session early and still
// saves the answers given so far. Like label, it never shows a model score, nor
// whether a job is in the test or the training set.
func review(ctx context.Context, conn *pgx.Conn) error {
	rows, err := conn.Query(ctx, `
	SELECT jobs.id, title, company, description FROM jobs JOIN labels ON jobs.id = labels.job_id WHERE would_apply = true ORDER BY job_id;
	`)
	if err != nil {
		return fmt.Errorf("failed to select labels rows: %w", err)
	}

	reader := bufio.NewReader(os.Stdin)
	reviews := []jobReview{}
	var (
		title       string
		company     string
		description string
	)
rowsLoop:
	for rows.Next() {
		review := jobReview{}
		err = rows.Scan(&review.jobID, &title, &company, &description)
		if err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan row: %w", err)
		}
		clearScreen()
		fmt.Printf("Title: %s\nCompany: %s\n\n%s\n", title, company, cleanDescription(description))
		fmt.Print("Keep (y/n): ")
	outer:
		for {
			keepStr, err := reader.ReadString('\n')
			if errors.Is(err, io.EOF) {
				// Ctrl+D: stop asking, but still save the answers given so far.
				break rowsLoop
			}
			if err != nil {
				rows.Close()
				return fmt.Errorf("read user input: %w", err)
			}
			keepStr = strings.TrimSpace(keepStr)
			switch keepStr {
			case "y":
				review.keep = true
				break outer
			case "n":
				review.keep = false
				break outer
			default:
				fmt.Printf("answer must be y or n!\n")
			}
		}
		reviews = append(reviews, review)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("failed to get jobs to review: %w", err)
	}
	// Free the connection before writing: after an early Ctrl+D the rows are still
	// open, and an Exec on a busy connection fails with "conn busy".
	rows.Close()
	for _, review := range reviews {
		if !review.keep {
			tag, err := conn.Exec(ctx, "UPDATE labels SET would_apply = false WHERE job_id = $1", review.jobID)
			if err != nil {
				return fmt.Errorf("failed to update label review: %w", err)
			}
			if tag.RowsAffected() != 1 {
				return fmt.Errorf("update label for job %d: expected 1 row changed, got %d", review.jobID, tag.RowsAffected())
			}
		}
	}
	return nil
}
