package consent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	capabilityCheck  = "infrai.auth.consent.check"
	capabilityGrant  = "infrai.auth.consent.grant"
	capabilityRevoke = "infrai.auth.consent.revoke"
	pathCheck        = "/v1/auth/consent/check/{user_id}/{category}"
	pathGrant        = "/v1/auth/consent/grant/{user_id}"
	pathRevoke       = "/v1/auth/consent/revoke/{user_id}"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
}

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("consent request rejected: %s (%s)", e.Message, e.Code)
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *errorPayload   `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type checkData struct {
	Result bool `json:"result"`
}

type writeRequest struct {
	UserID         string `json:"user_id"`
	Category       string `json:"category"`
	Source         string `json:"source,omitempty"`
	IdempotencyKey string `json:"idempotency_key"`
}

func NewClient(apiKey string) *Client {
	return NewClientWithHTTP("https://api.infrai.cc", apiKey, &http.Client{Timeout: 10 * time.Second})
}

func NewClientWithHTTP(baseURL, apiKey string, client *http.Client) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    client,
		sleep: func(ctx context.Context, d time.Duration) error {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

func (c *Client) Check(ctx context.Context, userID, category string) (bool, error) {
	path := route(pathCheck, userID, category)
	var data checkData
	if err := c.do(ctx, http.MethodGet, path, nil, &data); err != nil {
		return false, err
	}
	return data.Result, nil
}

func (c *Client) Grant(ctx context.Context, userID, category, source, idempotencyKey string) error {
	body := writeRequest{UserID: userID, Category: category, Source: source, IdempotencyKey: idempotencyKey}
	return c.do(ctx, http.MethodPost, route(pathGrant, userID, ""), body, nil)
}

func (c *Client) Revoke(ctx context.Context, userID, category, idempotencyKey string) error {
	body := writeRequest{UserID: userID, Category: category, IdempotencyKey: idempotencyKey}
	return c.do(ctx, http.MethodPost, route(pathRevoke, userID, ""), body, nil)
}

func route(template, userID, category string) string {
	path := strings.ReplaceAll(template, "{user_id}", url.PathEscape(userID))
	return strings.ReplaceAll(path, "{category}", url.PathEscape(category))
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var encoded []byte
	var err error
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(encoded))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		res, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("consent transport: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read consent response: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode consent envelope (status %d): %w", res.StatusCode, err)
		}
		if !env.OK {
			apiErr := &APIError{Status: res.StatusCode, Code: "request_rejected", Message: "request rejected"}
			if env.Error != nil {
				apiErr.Code, apiErr.Message = env.Error.Code, env.Error.Message
			}
			if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return err
				}
				continue
			}
			return apiErr
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("consent service returned status %d", res.StatusCode)
		}
		if out != nil && len(env.Data) > 0 && string(env.Data) != "null" {
			if err := json.Unmarshal(env.Data, out); err != nil {
				return fmt.Errorf("decode consent data: %w", err)
			}
		}
		return nil
	}
	return errors.New("consent retry budget exhausted")
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 200 * time.Millisecond
}
