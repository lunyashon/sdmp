package ratelimit

import (
	"testing"
	"time"
)

func TestAllowSeparateKeys(t *testing.T) {
	lim := New(1, 1)
	if ok, _ := lim.Allow("a"); !ok {
		t.Fatal("first a")
	}
	if ok, _ := lim.Allow("b"); !ok {
		t.Fatal("first b")
	}
	if ok, retry := lim.Allow("a"); ok {
		t.Fatalf("second a should wait, retry=%s", retry)
	}
}

func TestAllowRefills(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	lim := New(10, 1)
	lim.now = func() time.Time { return now }

	if ok, _ := lim.Allow("t"); !ok {
		t.Fatal("first")
	}
	if ok, _ := lim.Allow("t"); ok {
		t.Fatal("burst exhausted")
	}
	now = now.Add(100 * time.Millisecond)
	if ok, _ := lim.Allow("t"); !ok {
		t.Fatal("expected refill")
	}
}

func TestNilLimiterAllows(t *testing.T) {
	var lim *Limiter
	if ok, _ := lim.Allow("x"); !ok {
		t.Fatal("nil limiter must allow")
	}
	if New(0, 1) != nil {
		t.Fatal("rps<=0 must be disabled")
	}
}
