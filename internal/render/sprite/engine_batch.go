package sprite

import (
	"context"
	"image"

	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/domain"
)

// NativeWarmBatchResult summarizes batch native warm encoding.
type NativeWarmBatchResult struct {
	ReadySizes   []int
	EncodedSizes []int
	FailedSizes  []int
	Encoded      int
	Last         RenderResult
}

// NativeWarmBatch encodes multiple missing native sizes from one PNG decode.
func (e *Engine) NativeWarmBatch(ctx context.Context, pokemon domain.Pokemon, sizes []int) NativeWarmBatchResult {
	if e == nil || len(sizes) == 0 {
		return NativeWarmBatchResult{}
	}
	var ready, missing []int
	for _, cells := range sizes {
		cells = ClampFrameSide(cells)
		if cells < MinFrameSize {
			continue
		}
		if e.HasNativeCache(ctx, pokemon, cells) {
			ready = append(ready, cells)
			continue
		}
		missing = append(missing, cells)
	}
	out := NativeWarmBatchResult{ReadySizes: ready}
	if len(missing) == 0 {
		return out
	}
	if len(missing) == 1 {
		res := e.NativeWarmRender(ctx, pokemon, missing[0])
		if res.Native {
			out.EncodedSizes = []int{missing[0]}
			out.Encoded = 1
		} else {
			out.FailedSizes = []int{missing[0]}
		}
		out.Last = res
		return out
	}

	rend := e.nativeRenderer()
	if rend == nil {
		res := e.NativeWarmRender(ctx, pokemon, missing[0])
		out.Last = res
		if res.Native {
			out.EncodedSizes = []int{missing[0]}
			out.Encoded = 1
		} else {
			out.FailedSizes = []int{missing[0]}
		}
		return out
	}

	ref := e.resolveNativeAssetRef(ctx, pokemon)
	var png []byte
	if !ref.Ready {
		res, _ := e.resolveNativeAsset(ctx, pokemon, pokemon.ID)
		if res.UsePlaceholder || len(res.PNG) == 0 {
			out.Last = e.placeholderResult(displayName(pokemon), FrameWidth, FrameHeight)
			out.FailedSizes = append(out.FailedSizes, missing...)
			return out
		}
		png = res.PNG
		ref.Hash = res.ContentHash
		ref.Ready = true
		ref.Kind = res.AssetKind
	} else {
		var err error
		png, err = e.readNativePNG(ctx, pokemon, ref)
		if err != nil || len(png) == 0 {
			out.Last = e.placeholderResult(displayName(pokemon), FrameWidth, FrameHeight)
			out.FailedSizes = append(out.FailedSizes, missing...)
			return out
		}
	}
	hash := e.ensureCanonicalHash(ctx, pokemon, ref, png)
	if !IsHexSHA256(hash) {
		out.Last = e.placeholderResult(displayName(pokemon), FrameWidth, FrameHeight)
		out.FailedSizes = append(out.FailedSizes, missing...)
		return out
	}

	endDecode := debuglog.Trace(debuglog.Render, "png.decode")
	img, err := DecodeImage(png)
	endDecode()
	if err != nil {
		debuglog.Renderf("nativewarm batch decode fail pokemon=%d err=%v", pokemon.ID, err)
		out.Last = e.placeholderResult(displayName(pokemon), FrameWidth, FrameHeight)
		out.FailedSizes = append(out.FailedSizes, missing...)
		return out
	}

	label := displayName(pokemon)
	assetKind := ref.Kind
	if assetKind == "" {
		assetKind = namedKindForTier(AssetTierHD)
	}

	for _, cells := range missing {
		if err := ctx.Err(); err != nil {
			break
		}
		fw, fh := FrameSizeFor(cells)
		opts := e.renderOptions(label, fw, fh, hash, QualityInteractive)
		if res, ok := e.encodeNativeFrameDirect(ctx, rend, img, opts, hash, assetKind, pokemon.ID); ok {
			out.EncodedSizes = append(out.EncodedSizes, cells)
			out.Encoded++
			out.Last = res
		} else {
			out.FailedSizes = append(out.FailedSizes, cells)
		}
	}
	return out
}

func (e *Engine) nativeRenderer() Renderer {
	if e == nil || e.Modes == nil {
		return nil
	}
	rend := e.Modes.Resolve(e.Caps, ModeImage)
	if rend == nil || rend.Name() != "image" || !rend.Supported(e.Caps) {
		return nil
	}
	return rend
}

func (e *Engine) readNativePNG(ctx context.Context, pokemon domain.Pokemon, ref AssetRef) ([]byte, error) {
	if e.Source == nil {
		return nil, nil
	}
	return e.Source.ReadAsset(ctx, pokemon, ref)
}

func (e *Engine) encodeNativeFrame(ctx context.Context, rend Renderer, img image.Image, opts RenderOptions, hash, assetKind string, pokemonID int) (RenderResult, bool) {
	if hit, result := e.lookupNativeFrame(ctx, rend, opts, hash, assetKind); hit {
		return result, true
	}
	return e.encodeNativeFrameDirect(ctx, rend, img, opts, hash, assetKind, pokemonID)
}

func (e *Engine) encodeNativeFrameDirect(ctx context.Context, rend Renderer, img image.Image, opts RenderOptions, hash, assetKind string, pokemonID int) (RenderResult, bool) {
	key := nativeCacheKey(hash, rend, opts.Width, opts.Height, opts.BackgroundColor, opts.SixelColors)
	endEncode := debuglog.Trace(debuglog.Render, "native.encode")
	frame, err := rend.Render(ctx, img, opts)
	endEncode()
	if err != nil || frame.IsEmpty() {
		return RenderResult{}, false
	}
	if key != "" {
		e.memory().Put(key, frame.Text)
		if e.Artefacts != nil && frame.Native {
			if err := e.Artefacts.StoreNative(ctx, key, frame.Text); err != nil {
				debuglog.Renderf("render.store.native err=%v key=%s", err, key)
			}
		}
	}
	debuglog.Renderf("render.batch.encode pokemon=%d frame=%dx%d", pokemonID, opts.Width, opts.Height)
	return RenderResult{
		ANSI: frame.Text, CacheKey: key, Backend: frame.Backend,
		Native: frame.Native, AssetKind: assetKind,
	}, frame.Native
}
