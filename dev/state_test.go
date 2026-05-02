package dev

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestStore_FanoutMultipleSubscribers(t *testing.T) {
	s := NewStore(0, nil)
	const n = 5
	chs := make([]chan Event, n)
	cancels := make([]func(), n)
	for i := 0; i < n; i++ {
		chs[i], cancels[i] = s.Subscribe()
	}
	t.Cleanup(func() {
		for _, c := range cancels {
			c()
		}
	})

	s.MarkBuildStart([]string{"file.go"})
	for i, ch := range chs {
		select {
		case ev := <-ch:
			if ev.Type != EventBuildStart {
				t.Errorf("subscriber %d: got %s, want %s", i, ev.Type, EventBuildStart)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d did not receive event", i)
		}
	}
}

func TestStore_DropOnSlowSubscriber(t *testing.T) {
	s := NewStore(0, nil)
	_, cancel := s.Subscribe() // never read; channel cap is 16
	defer cancel()

	for i := 0; i < 100; i++ {
		s.MarkBuildStart([]string{"x.go"}) // would deadlock if broadcast blocked on the slow chan
	}
	// no assertion needed: we made it here without timeout / deadlock
}

func TestStore_CancelDuringBroadcast(t *testing.T) {
	// Race-detector regression test for #9 in the code review.
	// Before the fix, having close(ch) live outside the same lock that gates
	// broadcast iteration could cause a `send on closed channel` panic.
	s := NewStore(0, nil)
	var wg sync.WaitGroup
	stop := make(chan struct{})

	// 4 producers
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					s.MarkBuildStart([]string{"a.go"})
				}
			}
		}()
	}

	// 4 churn goroutines: subscribe + drain + cancel in a loop
	var got int64
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				ch, cancel := s.Subscribe()
				dctx := time.NewTimer(20 * time.Millisecond)
			drain:
				for {
					select {
					case <-ch:
						atomic.AddInt64(&got, 1)
					case <-dctx.C:
						break drain
					}
				}
				cancel()
			}
		}()
	}

	time.Sleep(150 * time.Millisecond)
	close(stop)
	wg.Wait()

	if got == 0 {
		t.Errorf("expected to receive at least one event during the run, got 0")
	}
}

func TestStore_MarkBuildOK_TransitionsAndBroadcasts(t *testing.T) {
	s := NewStore(0, nil)
	ch, cancel := s.Subscribe()
	defer cancel()

	s.MarkBuildStart([]string{"x.go"})
	<-ch // drain start
	s.MarkBuildOK("/tmp/bin")

	select {
	case ev := <-ch:
		if ev.Type != EventBuildOK {
			t.Fatalf("got %s, want %s", ev.Type, EventBuildOK)
		}
		if bp, _ := ev.Payload["binary_path"].(string); bp != "/tmp/bin" {
			t.Errorf("binary_path payload missing or wrong: %v", ev.Payload)
		}
	case <-time.After(time.Second):
		t.Fatal("no build:ok received")
	}

	snap := s.Snapshot()
	if snap.Build.Phase != BuildPhaseOK {
		t.Errorf("phase = %s, want ok", snap.Build.Phase)
	}
	if snap.Build.DurationMS < 0 {
		t.Errorf("duration_ms negative: %d", snap.Build.DurationMS)
	}
}

func TestStore_MarkBuildFail_KeepsErrors(t *testing.T) {
	s := NewStore(0, nil)
	s.MarkBuildStart([]string{"x.go"})
	errs := []StructuredError{{File: "a.go", Line: 1, Type: ErrorTypeSyntax, Message: "bad"}}
	s.MarkBuildFail(errs)
	snap := s.Snapshot()
	if snap.Build.Phase != BuildPhaseFail {
		t.Errorf("phase = %s, want failed", snap.Build.Phase)
	}
	if len(snap.CompileErrors) != 1 || snap.CompileErrors[0].File != "a.go" {
		t.Errorf("compile_errors not preserved: %+v", snap.CompileErrors)
	}
}
