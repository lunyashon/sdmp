package ratelimit

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestPaceNilAllows(t *testing.T) {
	var p *Pace
	if err := p.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if NewPace(0) != nil {
		t.Fatal("rps<=0 must disable")
	}
}

func TestPaceSpacesCalls(t *testing.T) {
	p := NewPace(20)
	start := time.Now()
	for i := 0; i < 4; i++ {
		if err := p.Wait(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	elapsed := time.Since(start)
	if elapsed < 100*time.Millisecond {
		t.Fatalf("elapsed %s, want >= 100ms", elapsed)
	}
}

func TestPaceWaitCanceled(t *testing.T) {
	p := NewPace(1)
	if err := p.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var wg sync.WaitGroup
	wg.Add(1)
	var got error
	go func() {
		defer wg.Done()
		got = p.Wait(ctx)
	}()
	wg.Wait()
	if got == nil {
		t.Fatal("expected cancel")
	}
}
