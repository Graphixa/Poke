package pokeapi

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrNotFound indicates the requested resource does not exist.
	ErrNotFound = errors.New("pokeapi: not found")
	// ErrOffline indicates the API could not be reached after retries.
	ErrOffline = errors.New("pokeapi: offline or unreachable")
)

// RateLimitError is returned when the API responds with HTTP 429 and retries
// are exhausted or not applicable.
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("pokeapi: rate limited; retry after %s", e.RetryAfter)
	}
	return "pokeapi: rate limited"
}

func (e *RateLimitError) Is(target error) bool {
	_, ok := target.(*RateLimitError)
	return ok
}

// StatusError captures unexpected HTTP statuses after validation.
type StatusError struct {
	StatusCode int
	Path       string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("pokeapi: unexpected status %d for %s", e.StatusCode, e.Path)
}
