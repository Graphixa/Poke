package uiimage

import (
	"image/color"

	"github.com/ploMP4/chafa-go"
)

const ChannelsRGBA = 4

// default cell pixel size used for aspect-aware geometry (common monospace).
const (
	defaultCellWidthPx  int32 = 10
	defaultCellHeightPx int32 = 20
)

// SymbolPreset selects which Unicode mosaic glyphs Chafa may use.
type SymbolPreset int

const (
	// SymbolsAll is the previous default (everything except "bad" glyphs).
	SymbolsAll SymbolPreset = iota
	// SymbolsDefault mirrors Chafa CLI defaults: block+border+space −wide −inverted.
	SymbolsDefault
	// SymbolsHalfQuad is best for pixel-art sharpness (half + quarter blocks).
	SymbolsHalfQuad
	// SymbolsMosaic adds sextant/octant for denser shape fidelity on modern fonts.
	SymbolsMosaic
)

// QualityPreset bundles Chafa knobs that most affect recognisability.
type QualityPreset int

const (
	// QualityBaseline matches the first chafa experiment (ALL symbols, stock knobs).
	QualityBaseline QualityPreset = iota
	// QualitySharp tunes for Pokémon pixel art in symbol mode.
	QualitySharp
	// QualityMosaic prioritises structural detail via denser Unicode mosaics.
	QualityMosaic
	// QualityMosaicFast keeps mosaic symbols but lowers work factor for TUI latency.
	QualityMosaicFast
)

// GenerateANSIBlock compiles raw pixel data into a TUI-compatible ANSI text block.
// Uses QualitySharp defaults for better sprite fidelity.
func GenerateANSIBlock(data *ImageData, targetWidthCells, targetHeightCells int) string {
	return GenerateANSIBlockOpts(data, RenderOpts{
		Width:   targetWidthCells,
		Height:  targetHeightCells,
		Mode:    chafa.CHAFA_CANVAS_MODE_TRUECOLOR,
		Quality: QualitySharp,
	})
}

// RenderOpts controls Chafa canvas geometry, colour mode, and quality knobs.
type RenderOpts struct {
	Width   int
	Height  int
	Mode    chafa.CanvasMode
	Stretch bool // if true, fill the full cell box (may distort)

	// Quality applies a named preset; individual fields below override when set.
	Quality QualityPreset

	Symbols         SymbolPreset
	WorkFactor      float32 // 0..1
	ColorSpace      chafa.ColorSpace
	ColorExtractor  chafa.ColorExtractor
	Dither          chafa.DitherMode
	CellWidthPx     int32
	CellHeightPx    int32
	Background      color.RGBA // if A>0, composite transparent pixels first
	TransparencyThr float32    // 0 = leave Chafa default; else alpha cutoff 0..1

	useSymbols        bool
	useWorkFactor     bool
	useColorSpace     bool
	useColorExtractor bool
	useDither         bool
}

// WithSymbols returns opts with an explicit symbol preset.
func (o RenderOpts) WithSymbols(s SymbolPreset) RenderOpts {
	o.Symbols = s
	o.useSymbols = true
	return o
}

// WithWorkFactor returns opts with an explicit Chafa work factor (0..1).
func (o RenderOpts) WithWorkFactor(f float32) RenderOpts {
	o.WorkFactor = f
	o.useWorkFactor = true
	return o
}

// GenerateANSIBlockOpts is the configurable form of GenerateANSIBlock.
func GenerateANSIBlockOpts(data *ImageData, opts RenderOpts) string {
	if data == nil || len(data.Pixels) == 0 {
		return ""
	}
	if opts.Width <= 0 || opts.Height <= 0 {
		return ""
	}
	if opts.Mode < 0 || opts.Mode >= chafa.CHAFA_CANVAS_MODE_MAX {
		opts.Mode = chafa.CHAFA_CANVAS_MODE_TRUECOLOR
	}

	opts = applyQualityPreset(opts)

	pixels := data.Pixels
	width, height := data.Width, data.Height
	if opts.Background.A > 0 {
		pixels = compositeOntoBackground(data, opts.Background)
	}

	symbolMap := chafa.SymbolMapNew()
	defer chafa.SymbolMapUnref(symbolMap)
	applySymbolPreset(symbolMap, opts.Symbols)

	config := chafa.CanvasConfigNew()
	defer chafa.CanvasConfigUnref(config)

	cellW, cellH := opts.CellWidthPx, opts.CellHeightPx
	if cellW <= 0 {
		cellW = defaultCellWidthPx
	}
	if cellH <= 0 {
		cellH = defaultCellHeightPx
	}
	chafa.CanvasConfigSetCellGeometry(config, cellW, cellH)

	w := int32(opts.Width)
	h := int32(opts.Height)
	if !opts.Stretch {
		fontRatio := float32(cellW) / float32(cellH)
		chafa.CalcCanvasGeometry(
			int32(width), int32(height),
			&w, &h,
			fontRatio,
			false, false,
		)
	}
	chafa.CanvasConfigSetGeometry(config, w, h)
	chafa.CanvasConfigSetSymbolMap(config, symbolMap)
	chafa.CanvasConfigSetCanvasMode(config, opts.Mode)
	chafa.CanvasConfigSetWorkFactor(config, opts.WorkFactor)
	chafa.CanvasConfigSetColorSpace(config, opts.ColorSpace)
	chafa.CanvasConfigSetColorExtractor(config, opts.ColorExtractor)
	chafa.CanvasConfigSetDitherMode(config, opts.Dither)
	// Preprocessing contrast boost is costly; leave it off for interactive browse.
	chafa.CanvasConfigSetPreprocessingEnabled(config, false)

	if opts.Background.A > 0 {
		chafa.CanvasConfigSetBgColor(config, packRGB(opts.Background))
	}
	if opts.TransparencyThr > 0 {
		chafa.CanvasConfigSetTransparencyThreshold(config, opts.TransparencyThr)
	}

	canvas := chafa.CanvasNew(config)
	defer chafa.CanvasUnRef(canvas)

	chafa.CanvasDrawAllPixels(
		canvas,
		chafa.CHAFA_PIXEL_RGBA8_UNASSOCIATED,
		pixels,
		int32(width),
		int32(height),
		int32(width*ChannelsRGBA),
	)

	gs := chafa.CanvasPrint(canvas, nil)
	if gs == nil {
		return ""
	}
	return gs.String()
}

func applyQualityPreset(opts RenderOpts) RenderOpts {
	switch opts.Quality {
	case QualitySharp:
		if !opts.useSymbols {
			opts.Symbols = SymbolsHalfQuad
		}
		if !opts.useWorkFactor {
			opts.WorkFactor = 1.0
		}
		if !opts.useColorSpace {
			opts.ColorSpace = chafa.CHAFA_COLOR_SPACE_DIN99D
		}
		if !opts.useColorExtractor {
			opts.ColorExtractor = chafa.CHAFA_COLOR_EXTRACTOR_MEDIAN
		}
		if !opts.useDither {
			opts.Dither = chafa.CHAFA_DITHER_MODE_NONE
		}
	case QualityMosaic:
		if !opts.useSymbols {
			opts.Symbols = SymbolsMosaic
		}
		if !opts.useWorkFactor {
			opts.WorkFactor = 1.0
		}
		if !opts.useColorSpace {
			opts.ColorSpace = chafa.CHAFA_COLOR_SPACE_DIN99D
		}
		if !opts.useColorExtractor {
			opts.ColorExtractor = chafa.CHAFA_COLOR_EXTRACTOR_MEDIAN
		}
		if !opts.useDither {
			opts.Dither = chafa.CHAFA_DITHER_MODE_NONE
		}
	case QualityMosaicFast:
		// Same mosaic glyph set; cheaper colour math for browse selection latency.
		if !opts.useSymbols {
			opts.Symbols = SymbolsMosaic
		}
		if !opts.useWorkFactor {
			opts.WorkFactor = 0.5
		}
		if !opts.useColorSpace {
			opts.ColorSpace = chafa.CHAFA_COLOR_SPACE_RGB
		}
		if !opts.useColorExtractor {
			opts.ColorExtractor = chafa.CHAFA_COLOR_EXTRACTOR_AVERAGE
		}
		if !opts.useDither {
			opts.Dither = chafa.CHAFA_DITHER_MODE_NONE
		}
	default: // QualityBaseline
		if !opts.useSymbols {
			opts.Symbols = SymbolsAll
		}
		if !opts.useWorkFactor {
			opts.WorkFactor = 0.5
		}
		if !opts.useColorSpace {
			opts.ColorSpace = chafa.CHAFA_COLOR_SPACE_RGB
		}
		if !opts.useColorExtractor {
			opts.ColorExtractor = chafa.CHAFA_COLOR_EXTRACTOR_AVERAGE
		}
		if !opts.useDither {
			opts.Dither = chafa.CHAFA_DITHER_MODE_NONE
		}
	}
	if opts.CellWidthPx == 0 {
		opts.CellWidthPx = defaultCellWidthPx
	}
	if opts.CellHeightPx == 0 {
		opts.CellHeightPx = defaultCellHeightPx
	}
	return opts
}

func applySymbolPreset(symbolMap *chafa.SymbolMap, preset SymbolPreset) {
	switch preset {
	case SymbolsDefault:
		chafa.SymbolMapApplySelectors(symbolMap, "block+border+space-wide-inverted")
	case SymbolsHalfQuad:
		// Maintainer tip for pixel art: half+quad with median extractor.
		chafa.SymbolMapApplySelectors(symbolMap, "half+quad+space")
	case SymbolsMosaic:
		chafa.SymbolMapApplySelectors(symbolMap, "block+half+quad+sextant+octant+border+space-wide-inverted")
	default:
		chafa.SymbolMapAddByTags(symbolMap, chafa.CHAFA_SYMBOL_TAG_ALL)
	}
}

func packRGB(c color.RGBA) uint32 {
	return uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B)
}

// compositeOntoBackground flattens alpha onto bg so Chafa sees opaque pixels.
func compositeOntoBackground(data *ImageData, bg color.RGBA) []uint8 {
	out := make([]uint8, len(data.Pixels))
	copy(out, data.Pixels)
	for i := 0; i+3 < len(out); i += 4 {
		a := out[i+3]
		if a == 255 {
			continue
		}
		if a == 0 {
			out[i] = bg.R
			out[i+1] = bg.G
			out[i+2] = bg.B
			out[i+3] = 255
			continue
		}
		as := float64(a) / 255
		ad := 1 - as
		out[i] = uint8(float64(out[i])*as + float64(bg.R)*ad + 0.5)
		out[i+1] = uint8(float64(out[i+1])*as + float64(bg.G)*ad + 0.5)
		out[i+2] = uint8(float64(out[i+2])*as + float64(bg.B)*ad + 0.5)
		out[i+3] = 255
	}
	return out
}
