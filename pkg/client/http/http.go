package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Config struct {
	Timeout       time.Duration
	MaxRetries    int
	RetryWaitMin  time.Duration
	RetryWaitMax  time.Duration
	RetryableFunc func(resp *http.Response, err error) bool
}

func DefaultConfig() *Config {
	return &Config{
		Timeout:       30 * time.Second,
		MaxRetries:    3,
		RetryWaitMin:  1 * time.Second,
		RetryWaitMax:  30 * time.Second,
		RetryableFunc: DefaultRetryable,
	}
}

func DefaultRetryable(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	return resp.StatusCode == 429 || resp.StatusCode >= 500
}

type Client struct {
	http   *http.Client
	config *Config
}

func New(timeout time.Duration) *Client {
	cfg := DefaultConfig()
	cfg.Timeout = timeout
	return &Client{http: &http.Client{Timeout: timeout}, config: cfg}
}

func NewWithConfig(cfg *Config) *Client {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Client{http: &http.Client{Timeout: cfg.Timeout}, config: cfg}
}

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

func (c *Client) backoff(attempt int) time.Duration {
	wait := c.config.RetryWaitMin * time.Duration(1<<uint(attempt-1))
	if wait > c.config.RetryWaitMax {
		wait = c.config.RetryWaitMax
	}
	return wait
}

func (c *Client) Get(ctx context.Context, url string, headers map[string]string) (*http.Response, error) {
	return c.Do(ctx, http.MethodGet, url, nil, headers)
}

func (c *Client) Post(ctx context.Context, url string, body []byte, headers map[string]string) (*http.Response, error) {
	return c.Do(ctx, http.MethodPost, url, body, headers)
}

func (c *Client) Put(ctx context.Context, url string, body []byte, headers map[string]string) (*http.Response, error) {
	return c.Do(ctx, http.MethodPut, url, body, headers)
}

func (c *Client) Delete(ctx context.Context, url string, headers map[string]string) (*http.Response, error) {
	return c.Do(ctx, http.MethodDelete, url, nil, headers)
}

func (c *Client) Patch(ctx context.Context, url string, body []byte, headers map[string]string) (*http.Response, error) {
	return c.Do(ctx, http.MethodPatch, url, body, headers)
}
