package connectors

import (
	"errors"
	"fmt"
	"time"
)

// RateLimitError means the source throttled us; the worker reschedules (snoozes)
// the job for RetryAfter without consuming a retry attempt (design-doc 0010).
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited, retry after %s", e.RetryAfter)
}

// AuthError means the source rejected our credentials (revoked/expired beyond
// refresh). It is terminal: the worker cancels the job and marks the connection
// needs_reauth rather than retrying forever (design-doc 0010).
type AuthError struct {
	Status int
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("source rejected credentials (status %d)", e.Status)
}

// StatusError is a non-2xx HTTP response that isn't a rate-limit or auth failure
// (those have their own types). Carrying the status code lets callers distinguish a
// permanent per-resource failure (404/410) from a transient one (5xx) — e.g. the
// issue-comment path degrades on the former and retries on the latter (design-doc
// 0011 P3-5).
type StatusError struct {
	Status int
	URL    string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("GET %s: status %d", e.URL, e.Status)
}

// TooLargeError means a response exceeded the artifact size cap — permanent for
// that resource (retrying won't shrink it).
type TooLargeError struct {
	URL   string
	Limit int
}

func (e *TooLargeError) Error() string {
	return fmt.Sprintf("GET %s: response exceeds %d bytes", e.URL, e.Limit)
}

// AsStatus reports a non-2xx HTTP status error and its code.
func AsStatus(err error) (*StatusError, bool) {
	var e *StatusError
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// AsRateLimit / AsAuth let the worker classify connector errors without importing
// internals.
func AsRateLimit(err error) (*RateLimitError, bool) {
	var e *RateLimitError
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

func AsAuth(err error) (*AuthError, bool) {
	var e *AuthError
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}
