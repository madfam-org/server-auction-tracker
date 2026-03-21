package scanner

import (
	"fmt"
	"math"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

// RetryClient wraps an HTTPClient with retry logic for transient failures.
type RetryClient struct {
	inner       HTTPClient
	maxAttempts int
	baseDelay   time.Duration
}

// NewRetryClient returns a client that retries on 5xx, 429, and network errors.
func NewRetryClient(inner HTTPClient, maxAttempts int, baseDelay time.Duration) *RetryClient {
	if inner == nil {
		inner = &http.Client{Timeout: 30 * time.Second}
	}
	if maxAttempts < 1 {
		maxAttempts = 3
	}
	if baseDelay <= 0 {
		baseDelay = 500 * time.Millisecond
	}
	return &RetryClient{
		inner:       inner,
		maxAttempts: maxAttempts,
		baseDelay:   baseDelay,
	}
}

func (c *RetryClient) Do(req *http.Request) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt < c.maxAttempts; attempt++ {
		resp, err := c.inner.Do(req)
		if err != nil {
			lastErr = err
			log.WithFields(log.Fields{
				"attempt": attempt + 1,
				"error":   err,
			}).Warn("HTTP request failed, retrying")
			c.sleep(attempt)
			continue
		}
		if !isRetryable(resp.StatusCode) {
			return resp, nil
		}
		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		resp.Body.Close() //nolint:errcheck
		log.WithFields(log.Fields{
			"attempt": attempt + 1,
			"status":  resp.StatusCode,
		}).Warn("Retryable HTTP status, retrying")
		c.sleep(attempt)
	}
	return nil, fmt.Errorf("max retries (%d) exhausted: %w", c.maxAttempts, lastErr)
}

func isRetryable(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= 500
}

func (c *RetryClient) sleep(attempt int) {
	delay := c.baseDelay * time.Duration(math.Pow(2, float64(attempt)))
	time.Sleep(delay)
}
