package ttlmap

import (
	"fmt"
	"testing"
	"time"
)

func TestTTLMap_CapExpiryAndSweep(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	m := New[string](2, func() time.Time { return now })

	if !m.Set("k1", "v1", time.Minute) {
		t.Fatal("expected first insert to succeed")
	}
	if !m.Set("k2", "v2", time.Minute) {
		t.Fatal("expected second insert to succeed")
	}
	if m.Set("k3", "v3", time.Minute) {
		t.Fatal("expected third insert to fail at cap")
	}
	if !m.Set("k1", "v1-updated", time.Minute) {
		t.Fatal("expected updating existing key at cap to succeed")
	}

	now = now.Add(61 * time.Second)
	if !m.Set("k3", "v3", time.Minute) {
		t.Fatal("expected insert after TTL expiry to sweep and succeed")
	}
	if got, ok := m.Get("k2"); ok || got != "" {
		t.Fatalf("expected expired k2 to be absent, got %q", got)
	}
	if removed := m.Sweep(); removed != 0 {
		t.Fatalf("expected 0 remaining expired entries, swept %d", removed)
	}

	for i := 0; i < 2; i++ {
		m.Set(fmt.Sprintf("s%d", i), "x", 5*time.Second)
	}
	now = now.Add(61 * time.Second)
	if removed := m.Sweep(); removed != 2 {
		t.Fatalf("expected Sweep to remove 2 expired entries, got %d", removed)
	}
	if m.Len() != 0 {
		t.Fatalf("expected empty map after Sweep, got %d", m.Len())
	}
}
