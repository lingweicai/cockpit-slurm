package account

import (
	"testing"
	"time"
)

func TestRequestCacheCachesCompletedResult(t *testing.T) {
	cache := NewRequestCache(time.Minute)
	if _, ok, wait := cache.Begin("req-1"); ok || wait != nil {
		t.Fatalf("Begin(req-1) returned ok=%v wait=%v, want reservation", ok, wait)
	}

	cache.Complete("req-1", RequestResult{Status: "ok"})
	got, ok := cache.Get("req-1")
	if !ok {
		t.Fatal("Get(req-1) = false, want true")
	}
	if got.Status != "ok" {
		t.Fatalf("Status = %q, want ok", got.Status)
	}

	cached, ok, wait := cache.Begin("req-1")
	if !ok || wait != nil {
		t.Fatalf("Begin(req-1) after complete returned ok=%v wait=%v, want cached", ok, wait)
	}
	if cached.Status != "ok" {
		t.Fatalf("cached status = %q, want ok", cached.Status)
	}
}

func TestRequestCacheTracksInFlightRequests(t *testing.T) {
	cache := NewRequestCache(time.Minute)
	if _, ok, wait := cache.Begin("req-2"); ok || wait != nil {
		t.Fatalf("first Begin(req-2) = ok=%v wait=%v, want reservation", ok, wait)
	}

	_, ok, wait := cache.Begin("req-2")
	if ok || wait == nil {
		t.Fatalf("second Begin(req-2) = ok=%v wait=%v, want in-flight wait", ok, wait)
	}

	cache.Complete("req-2", RequestResult{Status: "ok"})
	select {
	case <-wait:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for in-flight completion")
	}
}
