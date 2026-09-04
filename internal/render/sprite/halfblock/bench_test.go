package halfblock_test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/png"
	"net/http"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
	spritechafa "github.com/Graphixa/Pokedex/internal/render/sprite/chafa"
	"github.com/Graphixa/Pokedex/internal/render/sprite/halfblock"
	spriteimage "github.com/Graphixa/Pokedex/internal/render/sprite/image"
)

func loadBenchImage(b *testing.B) image.Image {
	b.Helper()
	url := sprite.CDNArtworkURL(25)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		b.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		b.Skip("network unavailable:", err)
	}
	defer resp.Body.Close()
	img, _, err := image.Decode(resp.Body)
	if err != nil {
		b.Fatal(err)
	}
	return img
}

func BenchmarkRenderers(b *testing.B) {
	img := loadBenchImage(b)
	sizes := []int{20, 28, 40}
	bg := sprite.DefaultBackground
	chafaR := spritechafa.Renderer{}
	halfR := halfblock.Renderer{}
	nativeR := spriteimage.New()

	for _, side := range sizes {
		w, h := sprite.FrameSizeFor(side)
		opts := sprite.RenderOptions{
			Width: w, Height: h,
			ColorMode:       sprite.ColorTrueColor,
			BackgroundColor: bg,
			Source:          sprite.SourceArtwork,
			AlphaThreshold:  16,
			Quality:         sprite.QualityInteractive,
		}
		b.Run(fmt.Sprintf("chafa/%dx%d", w, h), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := chafaR.Render(context.Background(), img, opts); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(fmt.Sprintf("halfblock/%dx%d", w, h), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := halfR.Render(context.Background(), img, opts); err != nil {
					b.Fatal(err)
				}
			}
		})
		if nativeR.Available() {
			b.Run(fmt.Sprintf("termimg/%dx%d", w, h), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := nativeR.Render(context.Background(), img, opts); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// Keep a compile-time check that PNG decode path works in tests.
func TestDecodeCDNArtworkSmoke(t *testing.T) {
	url := sprite.CDNArtworkURL(25)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Skip(err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	if buf.Len() < 100 {
		t.Fatalf("short body %d", buf.Len())
	}
}
