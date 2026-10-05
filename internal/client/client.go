package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	HTTP *http.Client
}

func NewClient(timeout time.Duration) *Client {
	return &Client{
		HTTP: &http.Client{Timeout: timeout},
	}
}

func (c *Client) HTTPGetBody(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch: %s; status: %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
