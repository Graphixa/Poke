package sprite_test

import (
	"testing"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

func TestFrameStepsThreeBuckets(t *testing.T) {
	want := []int{24, 32, 40}
	got := sprite.FrameSteps()
	if len(got) != len(want) {
		t.Fatalf("steps=%v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("steps[%d]=%d want %d (full=%v)", i, got[i], want[i], got)
		}
	}
}

func TestFrameStepsMaxFirstThreeBuckets(t *testing.T) {
	want := []int{40, 32, 24}
	got := sprite.FrameStepsMaxFirst()
	if len(got) != len(want) {
		t.Fatalf("steps=%v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("steps[%d]=%d want %d", i, got[i], want[i])
		}
	}
}

func TestClampFrameSideThreeBuckets(t *testing.T) {
	cases := map[int]int{
		20: 24, 23: 24, 24: 24, 25: 24, 31: 24,
		32: 32, 33: 32, 39: 32, 40: 40, 56: 40,
	}
	for in, want := range cases {
		if got := sprite.ClampFrameSide(in); got != want {
			t.Fatalf("ClampFrameSide(%d)=%d want %d", in, got, want)
		}
	}
}

func TestLargestFrameFittingThreeBuckets(t *testing.T) {
	steps := sprite.FrameSteps()
	if len(steps) != 3 {
		t.Fatalf("steps=%v", steps)
	}
	minOW, minOH := sprite.OuterFor(sprite.MinFrameSize)
	if sprite.LargestFrameFitting(minOW-1, minOH) != 0 {
		t.Fatal("expected no fit when too small")
	}
	if got := sprite.LargestFrameFitting(minOW, minOH); got != 24 {
		t.Fatalf("min fit=%d want 24", got)
	}
	maxOW, maxOH := sprite.OuterFor(40)
	if got := sprite.LargestFrameFitting(maxOW, maxOH); got != 40 {
		t.Fatalf("max fit=%d want 40", got)
	}
}

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
	w, h := sprite.FrameSizeFor(32)
	if w != 32 || h != 16 {
		t.Fatalf("got %dx%d", w, h)
	}
	w, h = sprite.FrameSizeFor(99)
	if w != sprite.MaxFrameSize || h != sprite.MaxFrameSize/sprite.CellAspectY {
		t.Fatalf("clamp got %dx%d", w, h)
	}
}

func TestOuterForVisualSquare(t *testing.T) {
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
