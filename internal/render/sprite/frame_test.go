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

func TestFrameStepsAndOuter(t *testing.T) {
	steps := sprite.FrameSteps()
	if len(steps) < 2 {
		t.Fatalf("steps=%v", steps)
	}
	if steps[0] != 16 || steps[len(steps)-1] != sprite.MaxFrameSize {
		t.Fatalf("steps=%v", steps)
	}
	ow, oh := sprite.OuterFor(16)
	if ow < 16 || oh < 8 {
		t.Fatalf("outer 16 = %dx%d", ow, oh)
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

func TestNormalizeBuckets(t *testing.T) {
	got := sprite.NormalizeBuckets([]int{40, 16, 16, 99, 8})
	want := []int{40, 16}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
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
