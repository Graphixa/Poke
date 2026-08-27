package image_test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"net/http"
	"testing"
	"time"

	"github.com/mattn/go-sixel"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
	spritechafa "github.com/Graphixa/Pokedex/internal/render/sprite/chafa"
	spriteimage "github.com/Graphixa/Pokedex/internal/render/sprite/image"
)

const (
	profileCellPxW = 8
	profileCellPxH = 16
)

func loadArtwork(t testing.TB) (pngBytes []byte, img image.Image) {
	t.Helper()
	url := sprite.CDNArtworkURL(25)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Skip("network:", err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	pngBytes = buf.Bytes()
	img, _, err = image.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatal(err)
	}
	return pngBytes, img
}

func median(ds []time.Duration) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	cp := append([]time.Duration(nil), ds...)
	for i := 0; i < len(cp); i++ {
		for j := i + 1; j < len(cp); j++ {
			if cp[j] < cp[i] {
				cp[i], cp[j] = cp[j], cp[i]
			}
		}
	}
	return cp[len(cp)/2]
}

func mean(ds []time.Duration) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	var sum time.Duration
	for _, d := range ds {
		sum += d
	}
	return sum / time.Duration(len(ds))
}

// TestSixelPipelineProfile breaks native browse cost into stages.
// Run: go test ./internal/render/sprite/image -run TestSixelPipelineProfile -v -count=1
func TestSixelPipelineProfile(t *testing.T) {
	pngBytes, img := loadArtwork(t)
	b := img.Bounds()
	t.Logf("source PNG: %d bytes, decoded %dx%d", len(pngBytes), b.Dx(), b.Dy())

	bg := color.RGBA{R: 0x31, G: 0x32, B: 0x44, A: 255}
	const iters = 40
	sides := []int{20, 28, 40}

	native := spriteimage.New()
	t.Logf("native protocol: %q available=%v", native.ProtocolName(), native.Available())

	for _, side := range sides {
		cw, ch := sprite.FrameSizeFor(side)
		pxW, pxH := cw*profileCellPxW, ch*profileCellPxH
		opts := sprite.RenderOptions{
			Width: cw, Height: ch,
			ColorMode:       sprite.ColorTrueColor,
			BackgroundColor: bg,
			Source:          sprite.SourceArtwork,
			AlphaThreshold:  16,
			Quality:         sprite.QualityInteractive,
		}

		var (
			decodeDs, prepDs, sixelDs, fullNativeDs, chafaDs []time.Duration
			sixelBytes, nativeBytes, chafaBytes             int
		)

		// Warm one pass so first-JIT noise is out of medians.
		{
			_, _, _ = image.Decode(bytes.NewReader(pngBytes))
			prepared := sprite.PrepareEncodeSrc(img, pxW, pxH, sprite.SourceArtwork, bg, 16, 1)
			var buf bytes.Buffer
			enc := sixel.NewEncoder(&buf)
			enc.Colors = 256
			_ = enc.Encode(prepared)
			_, _ = native.Render(context.Background(), img, opts)
			_, _ = (spritechafa.Renderer{}).Render(context.Background(), img, opts)
		}

		for i := 0; i < iters; i++ {
			t0 := time.Now()
			decoded, _, err := image.Decode(bytes.NewReader(pngBytes))
			if err != nil {
				t.Fatal(err)
			}
			decodeDs = append(decodeDs, time.Since(t0))

			// PrepareEncodeSrc: square artwork skips ContentBounds; downsample then Prepare.
			t0 = time.Now()
			prepared := sprite.PrepareEncodeSrc(decoded, pxW, pxH, sprite.SourceArtwork, bg, 16, 1)
			prepDs = append(prepDs, time.Since(t0))

			t0 = time.Now()
			var buf bytes.Buffer
			enc := sixel.NewEncoder(&buf)
			enc.Colors = 256
			if err := enc.Encode(prepared); err != nil {
				t.Fatal(err)
			}
			sixelDs = append(sixelDs, time.Since(t0))
			sixelBytes = buf.Len()

			t0 = time.Now()
			frame, err := native.Render(context.Background(), img, opts)
			fullNativeDs = append(fullNativeDs, time.Since(t0))
			if err == nil {
				nativeBytes = len(frame.Text)
			}

			t0 = time.Now()
			cf, err := (spritechafa.Renderer{}).Render(context.Background(), img, opts)
			chafaDs = append(chafaDs, time.Since(t0))
			if err == nil {
				chafaBytes = len(cf.Text)
			}
		}

		stageSum := mean(decodeDs) + mean(prepDs) + mean(sixelDs)
		t.Logf("── cells %dx%d (side=%d) → raster %dx%d ──", cw, ch, side, pxW, pxH)
		t.Logf("  PNG decode          mean=%8s  med=%8s", mean(decodeDs).Round(time.Microsecond), median(decodeDs).Round(time.Microsecond))
		t.Logf("  PrepareEncodeSrc    mean=%8s  med=%8s", mean(prepDs).Round(time.Microsecond), median(prepDs).Round(time.Microsecond))
		t.Logf("  go-sixel Encode     mean=%8s  med=%8s  out=%dB", mean(sixelDs).Round(time.Microsecond), median(sixelDs).Round(time.Microsecond), sixelBytes)
		t.Logf("  ── stage sum        mean=%8s", stageSum.Round(time.Microsecond))
		t.Logf("  full native.Render  mean=%8s  med=%8s  out=%dB  proto=%s",
			mean(fullNativeDs).Round(time.Microsecond), median(fullNativeDs).Round(time.Microsecond), nativeBytes, native.ProtocolName())
		t.Logf("  full chafa.Render   mean=%8s  med=%8s  out=%dB",
			mean(chafaDs).Round(time.Microsecond), median(chafaDs).Round(time.Microsecond), chafaBytes)

		sixelShare := float64(mean(sixelDs)) / float64(stageSum) * 100
		t.Logf("  sixel share of stage sum: %.0f%%", sixelShare)
	}
}

func BenchmarkSixelStages(b *testing.B) {
	_, img := loadArtwork(b)
	bg := sprite.DefaultBackground
	cw, ch := sprite.FrameSizeFor(40)
	pxW, pxH := cw*profileCellPxW, ch*profileCellPxH
	prepared := sprite.PrepareEncodeSrc(img, pxW, pxH, sprite.SourceArtwork, bg, 16, 1)

	b.Run(fmt.Sprintf("PrepareEncodeSrc/%dx%d", pxW, pxH), func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = sprite.PrepareEncodeSrc(img, pxW, pxH, sprite.SourceArtwork, bg, 16, 1)
		}
	})
	b.Run(fmt.Sprintf("SixelEncode/%dx%d", pxW, pxH), func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			enc := sixel.NewEncoder(&buf)
			enc.Colors = 256
			if err := enc.Encode(prepared); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run(fmt.Sprintf("SixelEncode_64colors/%dx%d", pxW, pxH), func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			enc := sixel.NewEncoder(&buf)
			enc.Colors = 64
			if err := enc.Encode(prepared); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run(fmt.Sprintf("SixelEncode_16colors/%dx%d", pxW, pxH), func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			enc := sixel.NewEncoder(&buf)
			enc.Colors = 16
			if err := enc.Encode(prepared); err != nil {
				b.Fatal(err)
			}
		}
	})
}
