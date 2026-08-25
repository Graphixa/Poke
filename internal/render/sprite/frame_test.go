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
