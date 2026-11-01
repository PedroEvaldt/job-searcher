package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const botUsername = "job-radar"

type discordWebhookBody struct {
	Content  string `json:"content"`
	Username string `json:"username"`
}

// sendNotification posts msg to a Discord webhook. Any non-2xx answer is an error;
// Discord answers a successful webhook call with 204 No Content.
func sendNotification(ctx context.Context, msg string, webhookURL string) error {
	body, err := json.Marshal(discordWebhookBody{
		Content:  msg,
		Username: botUsername,
	})
	if err != nil {
		return fmt.Errorf("encode discord payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build discord request: %w", redactURL(err))
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post to discord webhook: %w", redactURL(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("discord webhook returned %d: %s", resp.StatusCode, detail)
	}
	return nil
}

// redactURL drops the URL that *url.Error carries in its message. The webhook URL IS
// the credential — anyone holding it can post to the channel — so it must never reach
// a log line.
func redactURL(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return fmt.Errorf("%s: %w", urlErr.Op, urlErr.Err)
	}
	return err
}
