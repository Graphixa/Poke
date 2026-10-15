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

// QualityPreset bundles Chafa knobs for mosaic sprite rendering.
type QualityPreset int

const (
	// QualityMosaic prioritises structural detail via denser Unicode mosaics.
	QualityMosaic QualityPreset = iota
	// QualityMosaicFast keeps mosaic symbols but lowers work factor for TUI latency.
	QualityMosaicFast
)

// RenderOpts controls Chafa canvas geometry, colour mode, and quality knobs.
type RenderOpts struct {
	Width   int
	Height  int
	Mode    chafa.CanvasMode
	Stretch bool // if true, fill the full cell box (may distort)

	Quality QualityPreset

	workFactor      float32
	colorSpace      chafa.ColorSpace
	colorExtractor  chafa.ColorExtractor
	dither          chafa.DitherMode
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
	chafa.SymbolMapApplySelectors(symbolMap, "block+half+quad+sextant+octant+border+space-wide-inverted")

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
	chafa.CanvasConfigSetWorkFactor(config, opts.workFactor)
	chafa.CanvasConfigSetColorSpace(config, opts.colorSpace)
	chafa.CanvasConfigSetColorExtractor(config, opts.colorExtractor)
	chafa.CanvasConfigSetDitherMode(config, opts.dither)
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
	case QualityMosaic:
		opts.workFactor = 1.0
		opts.colorSpace = chafa.CHAFA_COLOR_SPACE_DIN99D
		opts.colorExtractor = chafa.CHAFA_COLOR_EXTRACTOR_MEDIAN
		opts.dither = chafa.CHAFA_DITHER_MODE_NONE
	default: // QualityMosaicFast
		opts.workFactor = 0.5
		opts.colorSpace = chafa.CHAFA_COLOR_SPACE_RGB
		opts.colorExtractor = chafa.CHAFA_COLOR_EXTRACTOR_AVERAGE
		opts.dither = chafa.CHAFA_DITHER_MODE_NONE
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
