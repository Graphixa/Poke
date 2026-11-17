package particle

import (
	"image/color"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/fireworks/vec"
)

func TestWorldScalePreservesBurstShape(t *testing.T) {
	// With g∝s² and v∝s, a horizontal shot should still travel ~same cells
	// relative to fall as an unscaled one — not collapse into a vertical drip.
	prev := WorldScale
	t.Cleanup(func() { WorldScale = prev })

	WorldScale = 0.5
	p := New(vec.Vec{X: 0, Y: 0}, vec.Vec{X: 200, Y: 0}, vec.Vec{}, color.RGBA{A: 255}, time.Second, 1)
	for range 40 {
		p.Update(30*time.Millisecond, 0.65, 0.28, 0, nil)
	}
	if p.Pos().X < 5 {
		t.Fatalf("radial travel too small (rain bug?): x=%v y=%v", p.Pos().X, p.Pos().Y)
	}
	if p.Pos().Y > p.Pos().X {
		t.Fatalf("gravity dominating burst (want x>=y early): x=%v y=%v", p.Pos().X, p.Pos().Y)
	}
}
