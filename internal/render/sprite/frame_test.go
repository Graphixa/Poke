package sprite_test

import (
	"testing"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

func TestFrameStepsBuckets(t *testing.T) {
	want := []int{16, 24, 32, 40}
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

func TestFrameStepsMaxFirstBuckets(t *testing.T) {
	want := []int{40, 32, 24, 16}
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

func TestClampFrameSideBuckets(t *testing.T) {
	cases := map[int]int{
		10: 16, 15: 16, 16: 16, 20: 16, 23: 16,
		24: 24, 25: 24, 31: 24,
		32: 32, 33: 32, 39: 32, 40: 40, 56: 40,
	}
	for in, want := range cases {
		if got := sprite.ClampFrameSide(in); got != want {
			t.Fatalf("ClampFrameSide(%d)=%d want %d", in, got, want)
		}
	}
}

func TestLargestFrameFittingBuckets(t *testing.T) {
	steps := sprite.FrameSteps()
	if len(steps) != 4 {
		t.Fatalf("steps=%v", steps)
	}
	minOW, minOH := sprite.OuterFor(16)
	if sprite.LargestFrameFitting(minOW-1, minOH) != 0 {
		t.Fatal("expected no fit when too small")
	}
	if got := sprite.LargestFrameFitting(minOW, minOH); got != 16 {
		t.Fatalf("min fit=%d want 16", got)
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
	if steps[0] != 16 || steps[len(steps)-1] != sprite.MaxFrameSize {
		t.Fatalf("steps=%v", steps)
	}
	ow, oh := sprite.OuterFor(16)
	if sprite.LargestFrameFitting(ow-1, oh) != 0 {
		t.Fatal("expected no fit when width short")
	}
	if got := sprite.LargestFrameFitting(ow, oh); got != 16 {
		t.Fatalf("got %d want 16", got)
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

func TestLargestArtFittingBuckets(t *testing.T) {
	if sprite.LargestArtFitting(15, 40) != 0 {
		t.Fatal("too narrow for 16")
	}
	if got := sprite.LargestArtFitting(16, 8); got != 16 {
		t.Fatalf("min art = %d want 16", got)
	}
	if got := sprite.LargestArtFitting(24, 12); got != 24 {
		t.Fatalf("24 pane = %d want 24", got)
	}
	if got := sprite.LargestArtFitting(88, 43); got != 40 {
		t.Fatalf("large pane = %d want 40", got)
	}
}

func TestLargestArtFittingPrefersNextBucket(t *testing.T) {
	if got := sprite.LargestArtFitting(40, 8); got != 16 {
		t.Fatalf("8 rows = %d want 16", got)
	}
	if got := sprite.LargestArtFitting(40, 15); got != 24 {
		t.Fatalf("15 rows = %d want 24", got)
	}
	if got := sprite.LargestArtFitting(40, 16); got != 32 {
		t.Fatalf("16 rows = %d want 32", got)
	}
	if got := sprite.LargestArtFitting(40, 19); got != 32 {
		t.Fatalf("19 rows = %d want 32", got)
	}
	if got := sprite.LargestArtFitting(40, 20); got != 40 {
		t.Fatalf("20 rows = %d want 40", got)
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
