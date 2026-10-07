package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	HTTP        *http.Client
	Retries     int
	InitialTime time.Duration
}

func NewClient(retries int, timeout, initialTime time.Duration) *Client {
	return &Client{
		HTTP:        &http.Client{Timeout: timeout},
		Retries:     retries,
		InitialTime: initialTime,
	}
}

func (c *Client) HTTPGetBody(ctx context.Context, url string) ([]byte, error) {
	wait := c.InitialTime
	for i := range c.Retries {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to build request: %w", err)
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read body: %w", err)
		}
		resp.Body.Close()

		switch code := resp.StatusCode; {
		case (code == http.StatusOK):
			return body, nil
		case (code >= 500 && code <= 599) || (code == http.StatusTooManyRequests):
			if i+1 == c.Retries {
				continue
			}
			t := time.NewTimer(wait)
			select {
			case <-t.C:
				wait *= 2
				t.Stop()
				continue
			case <-ctx.Done():
				t.Stop()
				return nil, fmt.Errorf("context canceled: %w", ctx.Err())
			}
		default:
			return nil, fmt.Errorf("failed to do req: %d", resp.StatusCode)
		}
	}
	return nil, fmt.Errorf("maximum number of retries made: %d", c.Retries)
}
