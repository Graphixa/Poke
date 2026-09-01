package coldsched_test

import (
	"context"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/render/sprite/coldsched"
)

func TestColdSchedulerSelectionDoesNotCancelActive(t *testing.T) {
	f := newFakeRenderer()
	f.setBlockAll(true)
	var displayed []string
	s := coldsched.NewScheduler(f, func(_ coldsched.Request, res coldsched.ColdResult) {
		displayed = append(displayed, res.Frame)
	})
	ctx := context.Background()

	s.Select(ctx, 1, poke(1), 32, false, false)
	time.Sleep(5 * time.Millisecond)
	s.Select(ctx, 2, poke(2), 32, false, false)
	s.Select(ctx, 3, poke(3), 32, false, false)
	s.Select(ctx, 4, poke(4), 32, false, false)
	s.Select(ctx, 5, poke(5), 32, false, false)

	if s.ActiveSpecies() != 1 {
		t.Fatalf("active=%d want 1 still running", s.ActiveSpecies())
	}
	if s.PendingSpecies() != 5 {
		t.Fatalf("pending=%d want 5 latest", s.PendingSpecies())
	}

	f.release(1)
	f.release(5)
	if err := s.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	for _, frame := range displayed {
		if frame == "frame-1:32" {
			t.Fatal("stale frame 1 must not display")
		}
	}
	if len(f.persisted) == 0 || f.persisted[0] != "1:32" {
		t.Fatalf("stale frame 1 should persist, persisted=%v", f.persisted)
	}
	lastStarted := f.started[len(f.started)-1]
	if lastStarted != 5 {
		t.Fatalf("next render=%d want 5", lastStarted)
	}
}

func TestColdSchedulerStaleCompletedFramePersists(t *testing.T) {
	f := newFakeRenderer()
	f.setBlockAll(true)
	s := newTestScheduler(f)
	ctx := context.Background()

	s.Select(ctx, 1, poke(1), 32, false, false)
	s.Select(ctx, 2, poke(2), 32, false, false)
	f.release(1)
	f.release(2)
	_ = s.WaitIdle(ctx)
	if len(f.persisted) == 0 {
		t.Fatal("expected stale valid frame persisted")
	}
}
