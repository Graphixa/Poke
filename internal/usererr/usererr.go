// Package usererr maps typed failures to concise user-facing copy and remediation.
package usererr

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/pokeapi"
)

// Format returns a concise user-facing error with a remediation hint when known.
func Format(err error) string {
	if err == nil {
		return ""
	}
	msg, hint := classify(err)
	if hint == "" {
		return msg
	}
	return msg + "\nHint: " + hint
}

// Message maps domain/network errors to short TUI/tab copy without stack noise.
func Message(err error) string {
	if err == nil {
		return ""
	}
	var rate *pokeapi.RateLimitError
	if errors.As(err, &rate) {
		if rate.RetryAfter > 0 {
			return fmt.Sprintf("Rate limited — retry after %s", rate.RetryAfter)
		}
		return "Rate limited — try again shortly"
	}
	if errors.Is(err, pokeapi.ErrOffline) || errors.Is(err, cache.ErrNotFound) {
		return "Unavailable offline — not in cache. Connect and reopen, or run poke update."
	}
	var offline *cache.OfflineError
	if errors.As(err, &offline) {
		return "Offline with empty cache — run poke update while online."
	}
	if errors.Is(err, cache.ErrCorrupt) {
		return "Corrupt cache entry quarantined — run poke update to rebuild."
	}
	if errors.Is(err, pokeapi.ErrNotFound) {
		return "Not found."
	}
	return err.Error()
}

func classify(err error) (msg, hint string) {
	msg = err.Error()

	type notFound interface {
		Error() string
		ExitCode() int
	}
	var nf notFound
	if errors.As(err, &nf) && strings.Contains(strings.ToLower(nf.Error()), "not found") {
		return nf.Error(), "Check the spelling or try poke search <query>."
	}

	var offline *cache.OfflineError
	if errors.As(err, &offline) {
		return offline.Error(), "Connect to the internet and run poke update, then retry."
	}

	var rate *pokeapi.RateLimitError
	if errors.As(err, &rate) {
		if rate.RetryAfter > 0 {
			return fmt.Sprintf("PokeAPI rate limited; retry after %s.", rate.RetryAfter),
				"Wait briefly, or continue browsing already-cached Pokémon offline."
		}
		return "PokeAPI rate limited.", "Wait briefly and retry; cached data remains usable."
	}

	if errors.Is(err, cache.ErrEmpty) {
		return cache.ErrEmpty.Error(), "Run poke update while online to bootstrap the cache."
	}
	if errors.Is(err, cache.ErrCorrupt) {
		return "Cached data was corrupt and has been quarantined.",
			"Run poke update to rebuild, or poke cache clear then poke update."
	}
	if errors.Is(err, pokeapi.ErrOffline) {
		return "Network unavailable.", "Use warm cache offline, or reconnect and run poke update."
	}
	if errors.Is(err, pokeapi.ErrNotFound) {
		return "Requested resource was not found on PokeAPI.", "Try another name or number."
	}

	lower := strings.ToLower(msg)
	if strings.Contains(lower, "rate limit") {
		return msg, "Wait briefly and retry; cached data remains usable."
	}
	if strings.Contains(lower, "offline") || strings.Contains(lower, "unavailable") {
		return msg, "Check connectivity, or browse already-cached entries."
	}
	return msg, ""
}
