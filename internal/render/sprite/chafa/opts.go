package chafa

import (
	"image/color"

	chafago "github.com/ploMP4/chafa-go"
)

const channelsRGBA = 4

// default cell pixel size used for aspect-aware geometry (common monospace).
const (
	defaultCellWidthPx  int32 = 10
	defaultCellHeightPx int32 = 20
)

// QualityPreset bundles Chafa knobs for mosaic sprite rendering.
type QualityPreset int

const (
	// QualityMosaic prioritises structural detail via denser Unicode mosaics.
	QualityMosaic QualityPreset = iota
	// QualityMosaicFast keeps mosaic symbols but lowers work factor (legacy / tests).
	QualityMosaicFast
)

const defaultSymbols = "block+half+quad+sextant+octant+border+space-wide-inverted"

// RenderOpts controls Chafa canvas geometry, colour mode, and quality knobs.
type RenderOpts struct {
	Width   int
	Height  int
	Mode    chafago.CanvasMode
	Stretch bool // if true, fill the full cell box (may distort)

	Quality QualityPreset

	workFactor      float32
	colorSpace      chafago.ColorSpace
	colorExtractor  chafago.ColorExtractor
	dither          chafago.DitherMode
	CellWidthPx     int32
	CellHeightPx    int32
	Background      color.RGBA // if A>0, composite transparent pixels first
	TransparencyThr float32    // 0 = leave Chafa default; else alpha cutoff 0..1
}

// GenerateANSIBlockOpts compiles raw pixel data into a TUI-compatible ANSI text block.
func GenerateANSIBlockOpts(data *ImageData, opts RenderOpts) string {
	if data == nil || len(data.Pixels) == 0 {
		return ""
	}
	if opts.Width <= 0 || opts.Height <= 0 {
		return ""
	}
	if opts.Mode < 0 || opts.Mode >= chafago.CHAFA_CANVAS_MODE_MAX {
		opts.Mode = chafago.CHAFA_CANVAS_MODE_TRUECOLOR
	}

	opts = applyQualityPreset(opts)

	pixels := data.Pixels
	width, height := data.Width, data.Height
	if opts.Background.A > 0 {
		pixels = compositeOntoBackground(data, opts.Background)
	}

	symbolMap := chafago.SymbolMapNew()
	defer chafago.SymbolMapUnref(symbolMap)
	chafago.SymbolMapApplySelectors(symbolMap, defaultSymbols)

	config := chafago.CanvasConfigNew()
	defer chafago.CanvasConfigUnref(config)

	cellW, cellH := opts.CellWidthPx, opts.CellHeightPx
	if cellW <= 0 {
		cellW = defaultCellWidthPx
	}
	if cellH <= 0 {
		cellH = defaultCellHeightPx
	}
	chafago.CanvasConfigSetCellGeometry(config, cellW, cellH)

	w := int32(opts.Width)
	h := int32(opts.Height)
	if !opts.Stretch {
		fontRatio := float32(cellW) / float32(cellH)
		chafago.CalcCanvasGeometry(
			int32(width), int32(height),
			&w, &h,
			fontRatio,
			false, false,
		)
	}
	chafago.CanvasConfigSetGeometry(config, w, h)
	chafago.CanvasConfigSetSymbolMap(config, symbolMap)
	chafago.CanvasConfigSetCanvasMode(config, opts.Mode)
	chafago.CanvasConfigSetWorkFactor(config, opts.workFactor)
	chafago.CanvasConfigSetColorSpace(config, opts.colorSpace)
	chafago.CanvasConfigSetColorExtractor(config, opts.colorExtractor)
	chafago.CanvasConfigSetDitherMode(config, opts.dither)
	chafago.CanvasConfigSetPreprocessingEnabled(config, false)

	if opts.Background.A > 0 {
		chafago.CanvasConfigSetBgColor(config, packRGB(opts.Background))
	}
	if opts.TransparencyThr > 0 {
		chafago.CanvasConfigSetTransparencyThreshold(config, opts.TransparencyThr)
	}

	canvas := chafago.CanvasNew(config)
	defer chafago.CanvasUnRef(canvas)

	chafago.CanvasDrawAllPixels(
		canvas,
		chafago.CHAFA_PIXEL_RGBA8_UNASSOCIATED,
		pixels,
		int32(width),
		int32(height),
		int32(width*channelsRGBA),
	)

	gs := chafago.CanvasPrint(canvas, nil)
	if gs == nil {
		return ""
	}
	return gs.String()
}

func applyQualityPreset(opts RenderOpts) RenderOpts {
	switch opts.Quality {
	case QualityMosaic:
		opts.workFactor = 1.0
		opts.colorSpace = chafago.CHAFA_COLOR_SPACE_DIN99D
		opts.colorExtractor = chafago.CHAFA_COLOR_EXTRACTOR_MEDIAN
		opts.dither = chafago.CHAFA_DITHER_MODE_NONE
	default: // QualityMosaicFast
		opts.workFactor = 0.5
		opts.colorSpace = chafago.CHAFA_COLOR_SPACE_RGB
		opts.colorExtractor = chafago.CHAFA_COLOR_EXTRACTOR_AVERAGE
		opts.dither = chafago.CHAFA_DITHER_MODE_NONE
	}
	if opts.CellWidthPx == 0 {
		opts.CellWidthPx = defaultCellWidthPx
	}
	if opts.CellHeightPx == 0 {
		opts.CellHeightPx = defaultCellHeightPx
	}
	return opts
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
