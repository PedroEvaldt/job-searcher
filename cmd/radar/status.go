package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func status(ctx context.Context, conn *pgx.Conn) error {
	var (
		toLabel int
		labeled int
	)
	err := conn.QueryRow(ctx, "SELECT COUNT(*) FILTER (WHERE l.labeled_at IS NULL) AS to_label, COUNT(*) FILTER (WHERE l.labeled_at IS NOT NULL) AS labeled FROM jobs j LEFT JOIN labels l ON j.id = l.job_id WHERE j.duplicate_of IS NULL;").Scan(&toLabel, &labeled)
	if err != nil {
		return fmt.Errorf("collect label statistic: %w", err)
	}
	fmt.Printf("LABELED %d/%d\n", labeled, toLabel)
	return nil
}
