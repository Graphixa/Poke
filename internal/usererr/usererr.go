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
	c := classify(err)
	if c.hint == "" {
		return c.format
	}
	return c.format + "\nHint: " + c.hint
}

// Message maps domain/network errors to short TUI/tab copy without stack noise.
func Message(err error) string {
	if err == nil {
		return ""
	}
	if c := classify(err); c.tab != "" {
		return c.tab
	}
	return err.Error()
}

type classification struct {
	format string
	hint   string
	tab    string
}

func classify(err error) classification {
	type notFound interface {
		Error() string
		ExitCode() int
	}
	var nf notFound
	if errors.As(err, &nf) && strings.Contains(strings.ToLower(nf.Error()), "not found") {
		return classification{
			format: nf.Error(),
			hint:   "Check the spelling or try poke search <query>.",
		}
	}

	var offline *cache.OfflineError
	if errors.As(err, &offline) {
		return classification{
			format: offline.Error(),
			hint:   "Connect to the internet and run poke update, then retry.",
			tab:    "Offline with empty cache — run poke update while online.",
		}
	}

	var rate *pokeapi.RateLimitError
	if errors.As(err, &rate) {
		tab := "Rate limited — try again shortly"
		format := "PokeAPI rate limited."
		if rate.RetryAfter > 0 {
			tab = fmt.Sprintf("Rate limited — retry after %s", rate.RetryAfter)
			format = fmt.Sprintf("PokeAPI rate limited; retry after %s.", rate.RetryAfter)
		}
		return classification{
			format: format,
			hint:   "Wait briefly and retry; cached data remains usable.",
			tab:    tab,
		}
	}

	offlineTab := "Unavailable offline — not in cache. Connect and reopen, or run poke update."
	if errors.Is(err, pokeapi.ErrOffline) {
		return classification{
			format: "Network unavailable.",
			hint:   "Use warm cache offline, or reconnect and run poke update.",
			tab:    offlineTab,
		}
	}
	if errors.Is(err, cache.ErrNotFound) {
		return classification{
			format: cache.ErrNotFound.Error(),
			tab:    offlineTab,
		}
	}

	if errors.Is(err, cache.ErrEmpty) {
		return classification{
			format: cache.ErrEmpty.Error(),
			hint:   "Run poke update while online to bootstrap the cache.",
		}
	}

	if errors.Is(err, cache.ErrCorrupt) {
		return classification{
			format: "Cached data was corrupt and has been quarantined.",
			hint:   "Run poke update to rebuild, or poke cache clear then poke update.",
			tab:    "Corrupt cache entry quarantined — run poke update to rebuild.",
		}
	}

	if errors.Is(err, pokeapi.ErrNotFound) {
		return classification{
			format: "Requested resource was not found on PokeAPI.",
			hint:   "Try another name or number.",
			tab:    "Not found.",
		}
	}

	msg := err.Error()
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "rate limit") {
		return classification{
			format: msg,
			hint:   "Wait briefly and retry; cached data remains usable.",
		}
	}
	if strings.Contains(lower, "offline") || strings.Contains(lower, "unavailable") {
		return classification{
			format: msg,
			hint:   "Check connectivity, or browse already-cached entries.",
		}
	}
	return classification{format: msg}
}
