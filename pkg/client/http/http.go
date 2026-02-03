// Package httpclient provides a robust HTTP client with retries and exponential backoff.
package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Config holds HTTP client settings.
type Config struct {
	Timeout       time.Duration
	MaxRetries    int
	RetryWaitMin  time.Duration
	RetryWaitMax  time.Duration
	RetryableFunc func(resp *http.Response, err error) bool
}

// DefaultConfig returns default settings: 30s timeout, 3 retries.
func DefaultConfig() *Config {
	return &Config{
		Timeout:       30 * time.Second,
		MaxRetries:    3,
		RetryWaitMin:  1 * time.Second,
		RetryWaitMax:  30 * time.Second,
		RetryableFunc: DefaultRetryable,
	}
}

// DefaultRetryable retries on errors, 429, and 5xx status codes.
func DefaultRetryable(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	return resp.StatusCode == 429 || resp.StatusCode >= 500
}

// Client is an HTTP client with retry support.
type Client struct {
	http   *http.Client
	config *Config
}

// New creates a client with the given timeout.
func New(timeout time.Duration) *Client {
	cfg := DefaultConfig()
	cfg.Timeout = timeout
	return &Client{http: &http.Client{Timeout: timeout}, config: cfg}
}

// NewWithConfig creates a client with custom config.
func NewWithConfig(cfg *Config) *Client {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Client{http: &http.Client{Timeout: cfg.Timeout}, config: cfg}
}

// Do executes an HTTP request with retries.
func (c *Client) Do(ctx context.Context, method, url string, body []byte, headers map[string]string) (*http.Response, error) {
	var resp *http.Response
	var err error
	var lastResp *http.Response

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			wait := c.backoff(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
		}

		var bodyReader io.Reader
		if body != nil {
			bodyReader = bytes.NewReader(body)
		}

		req, reqErr := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if reqErr != nil {
			return nil, reqErr
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err = c.http.Do(req)
		if !c.config.RetryableFunc(resp, err) {
			return resp, err
		}

		lastResp = resp
		if resp != nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}

	if err != nil {
		return nil, fmt.Errorf("request failed after %d retries: %w", c.config.MaxRetries, err)
	}
	if lastResp != nil {
		return nil, fmt.Errorf("request failed after %d retries: status %d", c.config.MaxRetries, lastResp.StatusCode)
	}
	return nil, fmt.Errorf("request failed after %d retries", c.config.MaxRetries)
}

// backoff calculates exponential wait time between retries.
func (c *Client) backoff(attempt int) time.Duration {
	wait := c.config.RetryWaitMin * time.Duration(1<<uint(attempt-1))
	if wait > c.config.RetryWaitMax {
		wait = c.config.RetryWaitMax
	}
	return wait
}

// Get performs an HTTP GET request.
func (c *Client) Get(ctx context.Context, url string, headers map[string]string) (*http.Response, error) {
	return c.Do(ctx, http.MethodGet, url, nil, headers)
}

// Post performs an HTTP POST request.
func (c *Client) Post(ctx context.Context, url string, body []byte, headers map[string]string) (*http.Response, error) {
	return c.Do(ctx, http.MethodPost, url, body, headers)
}

// Put performs an HTTP PUT request.
func (c *Client) Put(ctx context.Context, url string, body []byte, headers map[string]string) (*http.Response, error) {
	return c.Do(ctx, http.MethodPut, url, body, headers)
}

// Delete performs an HTTP DELETE request.
func (c *Client) Delete(ctx context.Context, url string, headers map[string]string) (*http.Response, error) {
	return c.Do(ctx, http.MethodDelete, url, nil, headers)
}

// Patch performs an HTTP PATCH request.
func (c *Client) Patch(ctx context.Context, url string, body []byte, headers map[string]string) (*http.Response, error) {
	return c.Do(ctx, http.MethodPatch, url, body, headers)
}
