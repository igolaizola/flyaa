package aa

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/google/uuid"
	"github.com/igolaizola/flyaa/pkg/fhttp"
)

type Client struct {
	client  func() (fhttp.Client, error)
	debug   bool
	baseURL string
}

type Config struct {
	Wait    time.Duration
	Debug   bool
	Proxy   string
	BaseURL string
}

func New(cfg *Config) (*Client, error) {
	// Validate input
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("aa: base url is required")
	}
	baseURL := strings.TrimRight(cfg.BaseURL, "/")

	// Create http client function
	client := func() (fhttp.Client, error) {
		return fhttp.NewClient(1*time.Minute, false, cfg.Proxy, cfg.Debug)
	}

	return &Client{
		baseURL: baseURL,
		client:  client,
		debug:   cfg.Debug,
	}, nil
}

var backoff = []time.Duration{
	100 * time.Millisecond,
	500 * time.Millisecond,
	1 * time.Second,
}

func (c *Client) do(ctx context.Context, method, path string, in, out any) ([]byte, error) {
	maxAttempts := len(backoff) + 1
	attempts := 0
	var err error
	for {
		if err != nil {
			slog.Debug("service: retrying request", "attempt", attempts+1, "error", err)
		}
		var b []byte
		b, err = c.doAttempt(ctx, method, path, in, out)
		if err == nil {
			return b, nil
		}
		// Increase attempts and check if we should stop
		attempts++
		if attempts >= maxAttempts {
			return nil, err
		}

		// Check if we should retry after waiting
		var retry bool
		var wait bool

		// Check for timeout error
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			retry = true
		}

		// Check status code
		var errStatus errStatusCode
		if errors.As(err, &errStatus) {
			switch int(errStatus) {
			case http.StatusBadGateway, http.StatusGatewayTimeout, http.StatusTooManyRequests,
				http.StatusInternalServerError, 520, 522,
				http.StatusForbidden, http.StatusBadRequest:
				// Retry on these status codes
				retry = true
				wait = true
			}
		}

		// Check for EOF errors or proxy errors
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
			strings.Contains(strings.ToLower(err.Error()), "proxy responded with non 200 code") {
			retry = true
		}

		// Stop if we shouldn't retry
		if !retry {
			return nil, err
		}

		// Wait before retrying if needed
		if wait {
			idx := attempts - 1
			if idx >= len(backoff) {
				idx = len(backoff) - 1
			}
			waitTime := backoff[idx]
			slog.Debug("service: waiting before retrying request", "waitTime", waitTime)
			t := time.NewTimer(waitTime)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-t.C:
			}
		}
	}
}

type errStatusCode int

func (e errStatusCode) Error() string {
	return fmt.Sprintf("%d", e)
}

func (c *Client) doAttempt(ctx context.Context, method, path string, in, out any) ([]byte, error) {
	// Prepare request body
	var body []byte
	var reqBody io.Reader
	var logBody string
	if in != nil {
		var err error
		body, err = json.Marshal(in)
		if err != nil {
			return nil, fmt.Errorf("service: couldn't marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(body)
		logBody = string(body)
	}
	slog.Debug("service: do", " method", method, "path", path, "body", logBody)

	// Create request
	path = strings.TrimPrefix(path, "/")
	u := fmt.Sprintf("%s/%s", c.baseURL, path)
	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return nil, fmt.Errorf("service: couldn't create request: %w", err)
	}
	c.addHeaders(req)

	// Do request
	client, err := c.client()
	if err != nil {
		return nil, fmt.Errorf("service: couldn't create http client: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("service: couldn't %s %s: %w", method, u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("service: couldn't read response body: %w", err)
	}
	logResp := string(respBody)
	slog.Debug("service: response", " method", method, "path", path, "status", resp.StatusCode, "body", logResp)

	// Check status code
	if resp.StatusCode != http.StatusOK {
		errMessage := string(respBody)
		if len(errMessage) > 100 {
			errMessage = errMessage[:100] + "..."
		}
		return nil, fmt.Errorf("service: %s %s returned (%s): %w", method, u, errMessage, errStatusCode(resp.StatusCode))
	}

	// Unmarshal response
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return nil, fmt.Errorf("service: couldn't unmarshal response body (%T): %w", out, err)
		}
	}
	return respBody, nil
}

func (c *Client) addHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-clientid", "MOBILE")
	req.Header.Set("Device-ID", uuid.New().String())
	req.Header.Set("User-Agent", "AAAndroid/2025.10")
	req.Header.Set("Version", "2025.10")
}
