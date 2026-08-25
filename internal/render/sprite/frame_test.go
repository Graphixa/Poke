package sprite_test

import (
	"testing"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

func TestFrameStepsAndFit(t *testing.T) {
	steps := sprite.FrameSteps()
	if len(steps) < 2 {
		t.Fatalf("steps=%v", steps)
	}
	if steps[0] != sprite.MinFrameSize || steps[len(steps)-1] != sprite.MaxFrameSize {
		t.Fatalf("steps=%v", steps)
	}
	ow, oh := sprite.OuterFor(sprite.MinFrameSize)
	if sprite.LargestFrameFitting(ow-1, oh) != 0 {
		t.Fatal("expected no fit when width short")
	}
	if got := sprite.LargestFrameFitting(ow, oh); got != sprite.MinFrameSize {
		t.Fatalf("got %d want min", got)
	}
	maxOW, maxOH := sprite.OuterFor(sprite.MaxFrameSize)
	if got := sprite.LargestFrameFitting(maxOW, maxOH); got != sprite.MaxFrameSize {
		t.Fatalf("got %d want max", got)
	}
}

func TestFrameSizeForAspect(t *testing.T) {
	w, h := sprite.FrameSizeFor(28)
	if w != 28 || h != 14 {
		t.Fatalf("got %dx%d", w, h)
	}
	w, h = sprite.FrameSizeFor(99)
	if w != sprite.MaxFrameSize || h != sprite.MaxFrameSize/sprite.CellAspectY {
		t.Fatalf("clamp got %dx%d", w, h)
	}
}

func TestOuterForVisualSquare(t *testing.T) {
	// Pad is aspect-corrected: outer box should stay visually square like the art.
	for _, side := range sprite.FrameSteps() {
		ow, oh := sprite.OuterFor(side)
		if ow != oh*sprite.CellAspectY {
			t.Fatalf("side %d outer %dx%d not visually square (want w=h*%d)", side, ow, oh, sprite.CellAspectY)
		}
		if ow-side != 2*sprite.FramePadX {
			t.Fatalf("side %d horizontal pad want %d got %d", side, 2*sprite.FramePadX, ow-side)
		}
		artH := side / sprite.CellAspectY
		if oh-artH != 2*sprite.FramePadY {
			t.Fatalf("side %d vertical pad want %d got %d", side, 2*sprite.FramePadY, oh-artH)
		}
	}
}
