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
	trieDuration := c.InitialTime
	for i := range c.Retries {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to build request: %w", err)
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch: %w", err)
		}

		switch {
		case (resp.StatusCode == http.StatusOK):
			defer resp.Body.Close()
			return io.ReadAll(resp.Body)
		case (resp.StatusCode >= 500 && resp.StatusCode <= 599) || (resp.StatusCode == http.StatusTooManyRequests):
			if (i + 1) == c.Retries {
				resp.Body.Close()
				break
			}
			t := time.NewTimer(trieDuration)
			select {
			case <-t.C:
				trieDuration *= 2
				resp.Body.Close()
				t.Stop()
				continue
			case <-ctx.Done():
				resp.Body.Close()
				t.Stop()
				return nil, fmt.Errorf("context canceled: %w", ctx.Err())
			}
		default:
			resp.Body.Close()
			return nil, fmt.Errorf("failed to do req: %d", resp.StatusCode)
		}
	}
	return nil, fmt.Errorf("maximum number of retries made: %d", c.Retries)
}
