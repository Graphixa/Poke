package assetwarm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

// ArtworkSizeThreshold is the estimate above which setup offers background download.
const ArtworkSizeThreshold int64 = 100 * 1024 * 1024 // 100 MiB

// FallbackBytesPerArtwork used when sampling fails (~120 KiB).
const FallbackBytesPerArtwork int64 = 120 * 1024

const defaultSampleSize = 16

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

// EstimateArtworkBytes samples official-artwork sizes and extrapolates to the full index.
// When sampling fails, uses FallbackBytesPerArtwork * species count.
func EstimateArtworkBytes(ctx context.Context, index []domain.SpeciesSummary, loader PokemonLoader, sampler SizeSampler) int64 {
	n := int64(len(index))
	if n == 0 {
		return 0
	}
	if sampler == nil || loader == nil {
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
	var sum int64
	var counted int64
	for i := 0; i < sampleN; i++ {
		if err := ctx.Err(); err != nil {
			break
		}
		idx := i * step
		if idx >= len(index) {
			idx = len(index) - 1
		}
		poke, err := loader.Pokemon(ctx, index[idx].ID)
		if err != nil {
			continue
		}
		url := sprite.URLForTier(poke, sprite.AssetTierHD)
		if url == "" {
			continue
		}
		sz, err := sampler.ProbeBytes(ctx, url)
		if err != nil || sz <= 0 {
			continue
		}
		sum += sz
		counted++
	}
	if counted == 0 {
		return n * FallbackBytesPerArtwork
	}
	avg := sum / counted
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
