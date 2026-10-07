package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the HTTP client of one source. It retries transient failures (429 and
// 5xx) with exponential backoff and keeps a minimum interval between requests, so
// one Client per source gives each source its own rate limit. It is not safe for
// concurrent use.
type Client struct {
	HTTP        *http.Client
	Retries     int           // total attempts per request, the first one included
	InitialWait time.Duration // backoff before the second attempt; doubles after each retry
	Interval    time.Duration // minimum gap between the end of a request and the start of the next

	lastRequest time.Time // end of the previous request; zero until the first one
}

// NewClient returns a Client whose individual HTTP requests time out after timeout.
func NewClient(retries int, timeout, initialWait, interval time.Duration) *Client {
	return &Client{
		HTTP:        &http.Client{Timeout: timeout},
		Retries:     retries,
		InitialWait: initialWait,
		Interval:    interval,
	}
}

// HTTPGetBody GETs url and returns the body of a 200 response. 429 and 5xx are
// tried again up to Retries attempts in total; any other status fails at once.
// Every wait, backoff or rate limit, returns early when ctx is done.
func (c *Client) HTTPGetBody(ctx context.Context, url string) ([]byte, error) {
	backoff := c.InitialWait
	var lastStatus int
	for i := range c.Retries {
		// Inside the loop on purpose: a retry is a request too. After a backoff
		// longer than the interval, nothing is left to wait here.
		if !c.lastRequest.IsZero() {
			if err := sleep(ctx, time.Until(c.lastRequest.Add(c.Interval))); err != nil {
				return nil, fmt.Errorf("waiting for rate limit: %w", err)
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to build request: %w", err)
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch: %w", err)
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close() // fully read (or failed to); a close error changes nothing
		c.lastRequest = time.Now()
		if readErr != nil {
			return nil, fmt.Errorf("failed to read body: %w", readErr)
		}

		switch code := resp.StatusCode; {
		case code == http.StatusOK:
			return body, nil
		case code == http.StatusTooManyRequests || code >= 500 && code <= 599:
			lastStatus = code
			if i+1 == c.Retries {
				continue
			}
			if err := sleep(ctx, backoff); err != nil {
				return nil, fmt.Errorf("waiting to retry: %w", err)
			}
			backoff *= 2
		default:
			return nil, fmt.Errorf("unexpected status %d from %s", code, url)
		}
	}
	return nil, fmt.Errorf("giving up after %d attempts: last status %d", c.Retries, lastStatus)
}

// sleep waits for d, or until ctx is done, whichever comes first. A d of zero or
// less returns at once.
func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
