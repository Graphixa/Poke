package pokeapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/pkg/version"
)

const (
	defaultBaseURL   = "https://pokeapi.co/api/v2"
	defaultTimeout   = 10 * time.Second
	defaultRetries   = 3
	maxResponseBytes = 8 << 20 // 8 MiB
	maxSpriteBytes   = 4 << 20 // 4 MiB
	listPageLimit    = 200

	// Idle connection pool sized for bulk CDN GETs (artwork warm).
	defaultMaxIdleConns        = 64
	defaultMaxIdleConnsPerHost = 32
)

// DefaultTransport returns an HTTP transport tuned for concurrent API + CDN fetches.
func DefaultTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = defaultMaxIdleConns
	t.MaxIdleConnsPerHost = defaultMaxIdleConnsPerHost
	t.ForceAttemptHTTP2 = true
	return t
}

// Client is a direct PokeAPI REST v2 client. Transport DTOs never leave this package.
type Client struct {
	baseURL    string
	httpClient *http.Client
	userAgent  string
	retries    int
	sleeper    func(context.Context, time.Duration) error
	randFloat  func() float64
}

// Options configures a Client. Zero values receive polite defaults from DECISIONS.md.
type Options struct {
	BaseURL    string
	Timeout    time.Duration
	Retries    int
	UserAgent  string
	HTTPClient *http.Client
	Sleeper    func(context.Context, time.Duration) error
	RandFloat  func() float64
}

func NewClient(opts Options) *Client {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	retries := opts.Retries
	if retries < 0 {
		retries = defaultRetries
	}
	baseURL := strings.TrimRight(opts.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	ua := strings.TrimSpace(opts.UserAgent)
	if ua == "" {
		ua = "poke-cli"
	}
	if !strings.Contains(ua, "/") {
		ua = ua + "/" + version.Current().Version
	}
	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout:   timeout,
			Transport: DefaultTransport(),
		}
	}
	sleeper := opts.Sleeper
	if sleeper == nil {
		sleeper = sleepContext
	}
	randFloat := opts.RandFloat
	if randFloat == nil {
		randFloat = rand.Float64
	}
	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
		userAgent:  ua,
		retries:    retries,
		sleeper:    sleeper,
		randFloat:  randFloat,
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Client) ListSpecies(ctx context.Context) ([]domain.SpeciesSummary, error) {
	var all []domain.SpeciesSummary
	next := fmt.Sprintf("%s/pokemon-species?limit=%d", c.baseURL, listPageLimit)
	for next != "" {
		var page namedResourceList
		if err := c.getJSON(ctx, next, &page); err != nil {
			return nil, err
		}
		for _, item := range page.Results {
			id, err := idFromURL(item.URL)
			if err != nil {
				return nil, fmt.Errorf("species list entry %q: %w", item.Name, err)
			}
			all = append(all, domain.SpeciesSummary{
				ID:          id,
				Slug:        item.Name,
				DisplayName: DisplayNameFromSlug(item.Name),
			})
		}
		next = page.Next
	}
	return all, nil
}

func (c *Client) GetPokemon(ctx context.Context, id int) (domain.Pokemon, error) {
	var dto pokemonDTO
	if err := c.getJSON(ctx, fmt.Sprintf("%s/pokemon/%d", c.baseURL, id), &dto); err != nil {
		return domain.Pokemon{}, err
	}
	return mapPokemon(dto)
}

func (c *Client) GetPokemonBySlug(ctx context.Context, slug string) (domain.Pokemon, error) {
	var dto pokemonDTO
	if err := c.getJSON(ctx, fmt.Sprintf("%s/pokemon/%s", c.baseURL, slug), &dto); err != nil {
		return domain.Pokemon{}, err
	}
	return mapPokemon(dto)
}

func (c *Client) GetSpecies(ctx context.Context, id int) (domain.Species, error) {
	var dto speciesDTO
	if err := c.getJSON(ctx, fmt.Sprintf("%s/pokemon-species/%d", c.baseURL, id), &dto); err != nil {
		return domain.Species{}, err
	}
	return mapSpecies(dto)
}

func (c *Client) GetEvolutionChain(ctx context.Context, id int) (domain.EvolutionTree, error) {
	var dto evolutionChainDTO
	if err := c.getJSON(ctx, fmt.Sprintf("%s/evolution-chain/%d", c.baseURL, id), &dto); err != nil {
		return domain.EvolutionTree{}, err
	}
	return mapEvolutionChain(dto), nil
}

func (c *Client) GetEncounters(ctx context.Context, pokemonID int) ([]domain.Encounter, error) {
	var dtos []encounterDTO
	if err := c.getJSON(ctx, fmt.Sprintf("%s/pokemon/%d/encounters", c.baseURL, pokemonID), &dtos); err != nil {
		return nil, err
	}
	return mapEncounters(dtos), nil
}

func (c *Client) GetMove(ctx context.Context, id int) (domain.Move, error) {
	var dto moveDTO
	if err := c.getJSON(ctx, fmt.Sprintf("%s/move/%d", c.baseURL, id), &dto); err != nil {
		return domain.Move{}, err
	}
	return mapMove(dto), nil
}

func (c *Client) GetSprite(ctx context.Context, spriteURL string) (io.ReadCloser, error) {
	body, err := c.getBytes(ctx, spriteURL, maxSpriteBytes)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(body)), nil
}

func (c *Client) getJSON(ctx context.Context, rawURL string, dest any) error {
	body, err := c.getBytes(ctx, rawURL, maxResponseBytes)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("decode %s: %w", rawURL, err)
	}
	return nil
}

func (c *Client) getBytes(ctx context.Context, rawURL string, maxBytes int64) ([]byte, error) {
	var lastErr error
	attempts := c.retries + 1
	for attempt := 0; attempt < attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		body, retryAfter, err := c.doGET(ctx, rawURL, maxBytes)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if errors.Is(err, ErrNotFound) {
			return nil, err
		}
		var status *StatusError
		if errors.As(err, &status) && status.StatusCode > 0 && status.StatusCode < 500 && status.StatusCode != http.StatusTooManyRequests {
			return nil, err
		}
		if attempt == attempts-1 {
			break
		}
		wait := c.backoff(attempt)
		if retryAfter > 0 {
			wait = retryAfter
		}
		if sleepErr := c.sleeper(ctx, wait); sleepErr != nil {
			return nil, sleepErr
		}
	}
	if isOffline(lastErr) {
		return nil, fmt.Errorf("%w: %v", ErrOffline, lastErr)
	}
	return nil, lastErr
}

func (c *Client) doGET(ctx context.Context, rawURL string, maxBytes int64) ([]byte, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json, image/png, */*")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, 0, err
	}
	if int64(len(body)) > maxBytes {
		return nil, 0, fmt.Errorf("pokeapi: response exceeds %d bytes", maxBytes)
	}

	switch {
	case resp.StatusCode == http.StatusOK:
		debuglog.Requestf("GET %s status=200 bytes=%d", sanitizeURL(rawURL), len(body))
		return body, 0, nil
	case resp.StatusCode == http.StatusNotFound:
		debuglog.Requestf("GET %s status=404", sanitizeURL(rawURL))
		return nil, 0, ErrNotFound
	case resp.StatusCode == http.StatusTooManyRequests:
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		debuglog.Requestf("GET %s status=429 retry_after=%s", sanitizeURL(rawURL), retryAfter)
		return nil, retryAfter, &RateLimitError{RetryAfter: retryAfter}
	case resp.StatusCode >= 500:
		debuglog.Requestf("GET %s status=%d", sanitizeURL(rawURL), resp.StatusCode)
		return nil, 0, &StatusError{StatusCode: resp.StatusCode, Path: rawURL}
	default:
		debuglog.Requestf("GET %s status=%d", sanitizeURL(rawURL), resp.StatusCode)
		return nil, 0, &StatusError{StatusCode: resp.StatusCode, Path: rawURL}
	}
}

func (c *Client) backoff(attempt int) time.Duration {
	// Bounded exponential backoff with jitter: 200ms * 2^attempt ± 50%.
	base := 200 * time.Millisecond
	exp := float64(base) * math.Pow(2, float64(attempt))
	jitter := 0.5 + c.randFloat() // 0.5–1.5
	d := time.Duration(exp * jitter)
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds < 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	if t, err := http.ParseTime(value); err == nil {
		d := time.Until(t)
		if d < 0 {
			return 0
		}
		return d
	}
	return 0
}

func isOffline(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrOffline) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "dial tcp")
}

// sanitizeURL strips query values that might carry secrets from debug lines.
func sanitizeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.RawQuery = ""
	u.Fragment = ""
	u.User = nil
	return u.String()
}

// Ensure Client satisfies the application API contract.
var _ interface {
	ListSpecies(context.Context) ([]domain.SpeciesSummary, error)
	GetPokemon(context.Context, int) (domain.Pokemon, error)
	GetSpecies(context.Context, int) (domain.Species, error)
	GetEvolutionChain(context.Context, int) (domain.EvolutionTree, error)
	GetEncounters(context.Context, int) ([]domain.Encounter, error)
	GetMove(context.Context, int) (domain.Move, error)
	GetSprite(context.Context, string) (io.ReadCloser, error)
} = (*Client)(nil)
