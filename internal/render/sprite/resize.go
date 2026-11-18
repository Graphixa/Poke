package sprite

import (
	"image"
	"image/color"
	"image/draw"

	xdraw "golang.org/x/image/draw"
)

// Resize maps src into tw×th using the algorithm appropriate for source kind.
// When fit is true, aspect ratio is preserved and the result is centred on bg.
// Artwork uses ApproxBiLinear (interactive default); call ResizeWithQuality for
// CatmullRom on prewarm.
func Resize(src *image.RGBA, tw, th int, source SpriteSource, fit bool, bg color.RGBA) *image.RGBA {
	return ResizeWithQuality(src, tw, th, source, QualityInteractive, fit, bg)
}

// ResizeWithQuality is Resize with an explicit quality profile for artwork scalers.
func ResizeWithQuality(src *image.RGBA, tw, th int, source SpriteSource, quality QualityProfile, fit bool, bg color.RGBA) *image.RGBA {
	if tw < 1 {
		tw = 1
	}
	if th < 1 {
		th = 1
	}
	if bg.A == 0 {
		bg = DefaultBackground
	}
	if !fit {
		return resizeFill(src, tw, th, source, quality)
	}
	return resizeFit(src, tw, th, source, quality, bg)
}

func resizeFill(src *image.RGBA, tw, th int, source SpriteSource, quality QualityProfile) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, tw, th))
	sb := src.Bounds()
	if sb.Empty() {
		return dst
	}
	scaler := scalerFor(source, quality)
	scaler.Scale(dst, dst.Bounds(), src, sb, xdraw.Src, nil)
	return dst
}

func resizeFit(src *image.RGBA, tw, th int, source SpriteSource, quality QualityProfile, bg color.RGBA) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, tw, th))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)

	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if sw <= 0 || sh <= 0 {
		return dst
	}

	hIfWidthLimited := sh * tw / sw
	wIfHeightLimited := sw * th / sh
	var dw, dh int
	if hIfWidthLimited <= th {
		dw, dh = tw, hIfWidthLimited
		if dh < 1 {
			dh = 1
		}
	} else {
		dw, dh = wIfHeightLimited, th
		if dw < 1 {
			dw = 1
		}
	}
	ox := (tw - dw) / 2
	oy := (th - dh) / 2

	fitted := image.NewRGBA(image.Rect(0, 0, dw, dh))
	scaler := scalerFor(source, quality)
	scaler.Scale(fitted, fitted.Bounds(), src, sb, xdraw.Src, nil)
	draw.Draw(dst, image.Rect(ox, oy, ox+dw, oy+dh), fitted, image.Point{}, draw.Src)
	return dst
}

func scalerFor(source SpriteSource, quality QualityProfile) xdraw.Interpolator {
	if source == SourceArtwork {
		if quality == QualityPrewarm {
			return xdraw.CatmullRom
		}
		// Interactive browse: ApproxBiLinear is much cheaper than CatmullRom.
		return xdraw.ApproxBiLinear
	}
	return xdraw.NearestNeighbor
}
