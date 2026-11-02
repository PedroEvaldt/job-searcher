package main

import (
	"context"
	"log/slog"
	"time"
)

func digestor(ctx context.Context, logger *slog.Logger, interval time.Duration, cycle func() error) {
	err := cycle()
	if err != nil {
		logger.Error("failed to run first digest cycle", "error", err)
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			err := cycle()
			if err != nil {
				logger.Error("failed to run digest cycle", "error", err)
			}
		case <-ctx.Done():
			logger.Info("shutting down")
			return
		}
	}
}
