package leads

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// DoWithRetry executes an HTTP request with automatic retry on transient errors
// (429 Too Many Requests, 503 Service Unavailable, and network errors).
//
// It uses req.Clone(ctx) on each attempt, so it works correctly for
// requests without a body (GET, HEAD). For requests with a body,
// ensure the body supports re-reading or use a factory pattern instead.
//
// Backoff doubles on each attempt, starting at 500 ms, capped at 30 s.
// A Retry-After header (in seconds) is honoured when present.
func DoWithRetry(ctx context.Context, client *http.Client, req *http.Request, maxRetries int) (*http.Response, error) {
	delay := 500 * time.Millisecond

	for attempt := 0; attempt <= maxRetries; attempt++ {
		clone := req.Clone(ctx)
		resp, err := client.Do(clone)
		if err != nil {
			if attempt == maxRetries {
				return nil, err
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
			delay = capDelay(delay * 2)
			continue
		}

		// On rate-limit or server overload, back off and retry.
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			resp.Body.Close()
			if attempt == maxRetries {
				return nil, fmt.Errorf("HTTP %d after %d retries: %s", resp.StatusCode, maxRetries, req.URL)
			}
			// Honour Retry-After if present
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, err2 := strconv.Atoi(ra); err2 == nil && secs > 0 {
					delay = time.Duration(secs) * time.Second
				}
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
			delay = capDelay(delay * 2)
			continue
		}

		return resp, nil
	}

	// Should not reach here, but be safe
	return nil, fmt.Errorf("max retries exceeded: %s", req.URL)
}

func capDelay(d time.Duration) time.Duration {
	const maxDelay = 30 * time.Second
	if d > maxDelay {
		return maxDelay
	}
	return d
}
