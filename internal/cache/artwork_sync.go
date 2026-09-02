package cache

import (
	"context"
	"fmt"
	"io"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

// ArtworkSyncResult summarizes a selective artwork sync pass.
type ArtworkSyncResult struct {
	New       int
	Updated   int
	Unchanged int
	Failed    int
	Changed   []int // species IDs whose PNG bytes changed
}

type spriteDownloader interface {
	GetSprite(ctx context.Context, spriteURL string) (io.ReadCloser, error)
}

// SyncArtwork downloads official artwork when missing or CDN URL changed.
// When URL matches and PNG exists, the file is left unchanged (fast path).
func (s *Service) SyncArtwork(ctx context.Context, index []domain.SpeciesSummary, progress ProgressFunc) (ArtworkSyncResult, error) {
	if s.api == nil {
		return ArtworkSyncResult{}, fmt.Errorf("cache: artwork sync requires API client")
	}
	dl, ok := s.api.(spriteDownloader)
	if !ok {
		return ArtworkSyncResult{}, fmt.Errorf("cache: API client cannot download sprites")
	}
	result := ArtworkSyncResult{}
	total := len(index)
	for i, sum := range index {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		cdnURL := sprite.CDNArtworkURL(sum.ID)
		if cdnURL == "" {
			result.Failed++
			reportArtworkSync(progress, i+1, total, sum.ID, StageArtwork)
			continue
		}
		hasPNG := s.store.HasNamedSprite(sum.ID, sum.Slug, SpriteKindArtwork)
		meta, metaErr := s.store.ReadNamedSpriteMeta(sum.ID, sum.Slug, SpriteKindArtwork)
		needDownload := !hasPNG
		if !needDownload && metaErr == nil && meta.URL != "" && meta.URL != cdnURL {
			needDownload = true
		}
		if !needDownload {
			result.Unchanged++
			reportArtworkSync(progress, i+1, total, sum.ID, StageArtwork)
			continue
		}
		data, err := downloadSprite(ctx, dl, cdnURL)
		if err != nil || len(data) == 0 || sprite.ValidatePNG(data) != nil {
			result.Failed++
			reportArtworkSync(progress, i+1, total, sum.ID, StageArtwork)
			continue
		}
		hash := sha256HexBytes(data)
		changed := !hasPNG || metaErr != nil || meta.SHA256 != hash
		if err := s.StoreNamedSpriteWithURL(ctx, sum.ID, sum.Slug, SpriteKindArtwork, cdnURL, hash, data); err != nil {
			result.Failed++
			reportArtworkSync(progress, i+1, total, sum.ID, StageArtwork)
			continue
		}
		if changed {
			if err := s.InvalidateNativeSpecies(sum.ID); err != nil {
				return result, err
			}
			result.Changed = append(result.Changed, sum.ID)
			if !hasPNG {
				result.New++
			} else {
				result.Updated++
			}
		} else {
			result.Unchanged++
		}
		reportArtworkSync(progress, i+1, total, sum.ID, StageArtwork)
	}
	return result, nil
}

func reportArtworkSync(progress ProgressFunc, done, total, speciesID int, stage Stage) {
	if progress == nil {
		return
	}
	progress(Progress{
		Stage:     stage,
		Done:      done,
		Total:     total,
		Message:   "Downloading artwork",
		SpeciesID: speciesID,
	})
}

func downloadSprite(ctx context.Context, dl spriteDownloader, url string) ([]byte, error) {
	rc, err := dl.GetSprite(ctx, url)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
