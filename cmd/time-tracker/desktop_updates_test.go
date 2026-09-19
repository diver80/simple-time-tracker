package main

import (
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gogpu/ui/app"

	"time-tracker/pkg/db"
	"time-tracker/pkg/timer"
	trackerui "time-tracker/pkg/ui"
)

func TestDesktopUpdatesDeferActionsAndCoalesceRedraw(t *testing.T) {
	wakes := 0
	u := &desktopUpdates{wake: func() { wakes++ }}
	var calls []int
	u.post(func() { calls = append(calls, 1) })
	u.post(func() { calls = append(calls, 2) })
	u.requestRedraw()
	if len(calls) != 0 || wakes != 3 {
		t.Fatalf("callbacks ran before flush or failed to wake: calls=%v wakes=%d", calls, wakes)
	}
	invalidations := 0
	invalidate := func() { invalidations++ }
	u.flush(invalidate)
	u.flush(invalidate)
	if !reflect.DeepEqual(calls, []int{1, 2}) || invalidations != 1 {
		t.Fatalf("calls=%v invalidations=%d", calls, invalidations)
	}
}

func TestDesktopUpdatesAllowReentrantActions(t *testing.T) {
	u := &desktopUpdates{wake: func() {}}
	calls := 0
	u.post(func() {
		calls++
		u.post(func() { calls++ })
		u.requestRedraw()
	})
	invalidations := 0
	invalidate := func() { invalidations++ }
	u.flush(invalidate)
	if calls != 1 {
		t.Fatalf("new action should be deferred to next flush: %d", calls)
	}
	u.flush(invalidate)
	if calls != 2 || invalidations != 2 {
		t.Fatalf("calls=%d invalidations=%d", calls, invalidations)
	}
}

func TestDesktopUpdatesConcurrentProducers(t *testing.T) {
	var wakes atomic.Int32
	u := &desktopUpdates{wake: func() { wakes.Add(1) }}
	const count = 100
	calls := 0 // Only accessed by the single consumer.
	var producers sync.WaitGroup
	for range count {
		producers.Go(func() {
			u.post(func() { calls++ })
			u.requestRedraw()
		})
	}
	producers.Wait()
	u.flush(func() {})
	if calls != count || wakes.Load() != 2*count {
		t.Fatalf("lost updates: calls=%d wakes=%d", calls, wakes.Load())
	}
}

func TestDesktopUpdatesInvalidateRetainedRoot(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	svc := timer.NewTimerService(repo)
	defer svc.Close()
	u := &desktopUpdates{wake: func() {}}
	root := trackerui.NewAppView(repo, svc, nil, u.requestRedraw)
	uiApp := app.New()
	uiApp.SetRoot(root)
	defer uiApp.Window().Close()
	uiApp.Window().HandleResize(420, 580)
	uiApp.Frame()
	// Simulate the compositor having cached the first frame.
	root.ClearRedraw()
	root.ClearSceneDirty()
	if root.IsSceneDirty() {
		t.Fatal("root should be clean before update")
	}
	u.requestRedraw()
	if root.IsSceneDirty() {
		t.Fatal("background request mutated the retained scene")
	}
	u.flush(func() {
		root.SetNeedsRedraw(true)
		uiApp.Window().Context().Invalidate()
	})
	if !root.IsSceneDirty() || !uiApp.Window().NeedsRedraw() {
		t.Fatal("redraw woke the window without invalidating its cached scene")
	}
}
