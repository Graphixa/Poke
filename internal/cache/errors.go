package cache

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound means the requested resource is absent from the local cache.
	ErrNotFound = errors.New("cache: not found")
	// ErrEmpty means no usable species index is present.
	ErrEmpty = errors.New("cache: empty; run poke refresh while online")
	// ErrCorrupt means an entry failed schema or JSON validation and was quarantined.
	ErrCorrupt = errors.New("cache: corrupt entry")
)

// OfflineError wraps a network failure that prevented bootstrap of an empty cache.
type OfflineError struct {
	Err error
}

func (e *OfflineError) Error() string {
	if e.Err == nil {
		return "cache is empty and the network is unavailable; connect and run poke refresh"
	}
	return fmt.Sprintf("cache is empty and the network is unavailable (%v); connect and run poke refresh", e.Err)
}

func (e *OfflineError) Unwrap() error {
	return e.Err
}
