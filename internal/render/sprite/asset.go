package sprite

import (
	"context"
	"errors"
	"sync"

	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/domain"
)

var errAssetMissing = errors.New("sprite: asset missing")

// AssetRef is metadata for an original artwork/pixel PNG. Resolving it must
// not require reading image bytes.
type AssetRef struct {
	PokemonID int
	Source    SpriteSource
	Hash      string
	Path      string
	URL       string
	Ready     bool
	Kind      string // "artwork" | "pixel"
}

var inflightURLs sync.Map // url -> *flightCall

// ResolveAsset returns original-asset metadata without reading PNG bytes.
func (r *SourceResolver) ResolveAsset(ctx context.Context, pokemon domain.Pokemon, tier AssetTier) AssetRef {
	kind := namedKindForTier(tier)
	src := SourceArtwork
	if tier == AssetTierPixel {
		src = SourcePixel
	}
	ref := AssetRef{PokemonID: pokemon.ID, Source: src, Kind: kind, URL: URLForTier(pokemon, tier)}
	if r == nil {
		return ref
	}
	if r.Store != nil && pokemon.ID != 0 {
		hash, path, ready, err := r.Store.NamedAssetMeta(ctx, pokemon.ID, pokemon.Slug, kind)
		if err == nil && ready && hash != "" {
			debuglog.Renderf("artwork.download.hit id=%d hash=%s", pokemon.ID, hash)
			ref.Hash = hash
			ref.Path = path
			ref.Ready = true
			return ref
		}
	}
	return ref
}

// ReadAsset returns original PNG bytes for a ready AssetRef.
func (r *SourceResolver) ReadAsset(ctx context.Context, pokemon domain.Pokemon, ref AssetRef) ([]byte, error) {
	notePNGRead()
	end := debuglog.Trace(debuglog.Render, "asset.read")
	defer end()
	if r == nil || !ref.Ready {
		return nil, errAssetMissing
	}
	if r.Store != nil && ref.Path != "" {
		if data, err := r.Store.ReadAssetBytes(ctx, ref.Path); err == nil && len(data) > 0 {
			return data, nil
		}
	}
	if r.Store != nil && pokemon.ID != 0 {
		kind := ref.Kind
		if kind == "" {
			kind = namedKindForTier(AssetTierPixel)
			if ref.Source == SourceArtwork {
				kind = namedKindForTier(AssetTierHD)
			}
		}
		return r.Store.NamedSprite(ctx, pokemon.ID, pokemon.Slug, kind)
	}
	return nil, errAssetMissing
}

// ResolvePrefersMeta tries metadata first, then falls back to byte Resolve.
func (r *SourceResolver) ResolvePrefersMeta(ctx context.Context, pokemon domain.Pokemon, tier AssetTier) (AssetRef, SourceResult) {
	ref := r.ResolveAsset(ctx, pokemon, tier)
	if ref.Ready && ref.Hash != "" {
		return ref, SourceResult{ContentHash: ref.Hash, URL: ref.URL, AssetKind: ref.Kind}
	}
	res := r.Resolve(ctx, pokemon, tier)
	if res.UsePlaceholder {
		return ref, res
	}
	src := SourceArtwork
	if tier == AssetTierPixel {
		src = SourcePixel
	}
	kind := res.AssetKind
	if kind == "" {
		kind = namedKindForTier(tier)
	}
	return AssetRef{
		PokemonID: pokemon.ID,
		Source:    src,
		Hash:      res.ContentHash,
		URL:       res.URL,
		Ready:     len(res.PNG) > 0,
		Kind:      kind,
	}, res
}

func downloadDeduped(ctx context.Context, api Downloader, url string) ([]byte, error) {
	if url == "" || api == nil {
		return nil, errAssetMissing
	}
	type slot struct {
		wg   sync.WaitGroup
		data []byte
		err  error
	}
	if v, ok := inflightURLs.Load(url); ok {
		s := v.(*slot)
		s.wg.Wait()
		return s.data, s.err
	}
	s := &slot{}
	s.wg.Add(1)
	actual, loaded := inflightURLs.LoadOrStore(url, s)
	if loaded {
		s = actual.(*slot)
		s.wg.Wait()
		return s.data, s.err
	}
	debuglog.Renderf("artwork.download.start url=%s", url)
	NoteHTTPArtwork()
	s.data, s.err = download(ctx, api, url)
	s.wg.Done()
	inflightURLs.Delete(url)
	return s.data, s.err
}
