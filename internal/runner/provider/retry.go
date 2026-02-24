package provider

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// retryDo executes fn up to maxRetries+1 times with delay between attempts.
// Returns the first successful response or the last error.
func retryDo(ctx context.Context, maxRetries int, delay time.Duration, fn func() (*http.Response, error)) (*http.Response, error) {
	var lastErr error
	attempts := maxRetries + 1
	if attempts < 1 {
		attempts = 1
	}

	for i := range attempts {
		resp, err := fn()
		if err == nil && resp.StatusCode < 500 && resp.StatusCode != 429 {
			return resp, nil
		}

		if err != nil {
			lastErr = err
		} else if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			_ = resp.Body.Close()
		}

		if i < attempts-1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return nil, fmt.Errorf("all %d attempts failed: %w", attempts, lastErr)
}

// sharedTransport is a connection-pooling HTTP transport shared across providers.
var sharedTransport = &http.Transport{
	MaxIdleConns:        100,
	MaxIdleConnsPerHost: 10,
	IdleConnTimeout:     90 * time.Second,
}

// newHTTPClient creates an HTTP client with connection pooling.
func newHTTPClient() *http.Client {
	return &http.Client{
		Transport: sharedTransport,
		Timeout:   5 * time.Minute,
	}
}
