// Package uiimage loads PNG pixel buffers and converts them with Chafa.
package uiimage

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/png"
	"os"
)

// ImageData wraps raw pixel state for the Chafa engine.
type ImageData struct {
	Pixels []uint8
	Width  int
	Height int
}

// LoadPNGAndExtractBuffer reads a PNG file and flattens it to an RGBA byte slice.
func LoadPNGAndExtractBuffer(filePath string) (*ImageData, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}
	return FromImage(img)
}

// FromPNG decodes PNG bytes into an RGBA pixel buffer.
func FromPNG(png []byte) (*ImageData, error) {
	if len(png) == 0 {
		return nil, fmt.Errorf("uiimage: empty png")
	}
	img, _, err := image.Decode(bytes.NewReader(png))
	if err != nil {
		return nil, fmt.Errorf("uiimage: decode: %w", err)
	}
	return FromImage(img)
}

// FromImage flattens any image.Image to tightly packed unassociated RGBA8.
func FromImage(img image.Image) (*ImageData, error) {
	if img == nil {
		return nil, fmt.Errorf("uiimage: nil image")
	}
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("uiimage: empty bounds")
	}

	// Always origin at (0,0) so Pix is a contiguous WxH RGBA buffer.
	rgbaImg := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(rgbaImg, rgbaImg.Bounds(), img, bounds.Min, draw.Src)

	return &ImageData{
		Pixels: rgbaImg.Pix,
		Width:  width,
		Height: height,
	}, nil
}
