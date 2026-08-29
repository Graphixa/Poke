package assetwarm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

// ArtworkSizeThreshold is the estimate above which setup offers background download.
const ArtworkSizeThreshold int64 = 100 * 1024 * 1024 // 100 MiB

// FallbackBytesPerArtwork used when sampling fails (~120 KiB).
const FallbackBytesPerArtwork int64 = 120 * 1024

const (
	defaultSampleSize    = 16
	estimateProbeWorkers = 8
)

// SizeSampler probes remote artwork sizes (typically via HEAD Content-Length).
type SizeSampler interface {
	ProbeBytes(ctx context.Context, url string) (int64, error)
}

// HTTPSizeSampler uses HEAD requests; falls back when Content-Length is missing.
type HTTPSizeSampler struct {
	Client *http.Client
}

func (s HTTPSizeSampler) ProbeBytes(ctx context.Context, url string) (int64, error) {
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.ContentLength > 0 {
		return resp.ContentLength, nil
	}
	return 0, io.EOF
}

// NeedsModePrompt reports whether estimatedBytes warrants Now vs Background choice.
func NeedsModePrompt(estimatedBytes int64) bool {
	return estimatedBytes >= ArtworkSizeThreshold
}

// ArtworkWorkers returns concurrency for ModeArtwork bulk download.
// Floors at 16 and caps at 24 so CDN GETs are not stuck at the JSON max_concurrency (8).
func ArtworkWorkers(maxConcurrency int) int {
	w := maxConcurrency
	if w < 16 {
		w = 16
	}
	if w > 24 {
		w = 24
	}
	return w
}

// PixelWorkers returns concurrency for ModePixel bulk download.
// Floors at 12 and caps at 20 — CDN-bound, independent of JSON sema.
func PixelWorkers(maxConcurrency int) int {
	w := maxConcurrency
	if w < 12 {
		w = 12
	}
	if w > 20 {
		w = 20
	}
	return w
}

// EstimateArtworkBytes samples official-artwork sizes and extrapolates to the full index.
// Uses CDN URLs from species IDs (no Pokemon JSON). When sampling fails, uses
// FallbackBytesPerArtwork * species count.
func EstimateArtworkBytes(ctx context.Context, index []domain.SpeciesSummary, loader PokemonLoader, sampler SizeSampler) int64 {
	n := int64(len(index))
	if n == 0 {
		return 0
	}
	if sampler == nil {
		return n * FallbackBytesPerArtwork
	}
	sampleN := defaultSampleSize
	if int(n) < sampleN {
		sampleN = int(n)
	}
	step := int(n) / sampleN
	if step < 1 {
		step = 1
	}

	type sample struct {
		id int
	}
	jobs := make([]sample, 0, sampleN)
	for i := 0; i < sampleN; i++ {
		idx := i * step
		if idx >= len(index) {
			idx = len(index) - 1
		}
		jobs = append(jobs, sample{id: index[idx].ID})
	}

	var sum atomic.Int64
	var counted atomic.Int64
	workers := estimateProbeWorkers
	if workers > len(jobs) {
		workers = len(jobs)
	}
	ch := make(chan sample, len(jobs))
	for _, j := range jobs {
		ch <- j
	}
	close(ch)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				if ctx.Err() != nil {
					return
				}
				url := sprite.CDNArtworkURL(j.id)
				// Optional loader override when CDN shape is unavailable.
				if loader != nil {
					if poke, err := loader.Pokemon(ctx, j.id); err == nil {
						if u := sprite.URLForTier(poke, sprite.AssetTierHD); u != "" {
							url = u
						}
					}
				}
				if url == "" {
					continue
				}
				sz, err := sampler.ProbeBytes(ctx, url)
				if err != nil || sz <= 0 {
					continue
				}
				sum.Add(sz)
				counted.Add(1)
			}
		}()
	}
	wg.Wait()

	c := counted.Load()
	if c == 0 {
		return n * FallbackBytesPerArtwork
	}
	avg := sum.Load() / c
	return avg * n
}

// FormatBytes returns a short human size for UI copy.
func FormatBytes(n int64) string {
	const (
		kb = 1024.0
		mb = 1024.0 * kb
	)
	switch {
	case float64(n) >= mb:
		return fmt.Sprintf("%.1f MB", float64(n)/mb)
	case float64(n) >= kb:
		return fmt.Sprintf("%.0f KB", float64(n)/kb)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
