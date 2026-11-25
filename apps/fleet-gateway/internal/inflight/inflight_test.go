package inflight

import (
	"testing"
)

func TestUserSlots(t *testing.T) {
	t.Run("nil never limits", func(t *testing.T) {
		var u *PerKey
		for range 5 {
			rel, ok := u.TryAcquire("user-key")
			if !ok {
				t.Fatal("nil *PerKey refused acquire")
			}
			rel()
			rel()
		}
	})

	t.Run("caps per key, idempotent release, deletes at zero", func(t *testing.T) {
		u := NewPerKey(2)
		rel1, ok := u.TryAcquire("u1")
		if !ok {
			t.Fatal("first acquire u1 failed")
		}
		rel2, ok := u.TryAcquire("u1")
		if !ok {
			t.Fatal("second acquire u1 failed")
		}
		if _, acquired := u.TryAcquire("u1"); acquired {
			t.Fatal("third acquire u1 succeeded above max=2")
		}

		relOther, ok := u.TryAcquire("u2")
		if !ok {
			t.Fatal("first acquire u2 failed while u1 was full")
		}

		// Releasing rel1 twice must only decrement once.
		rel1()
		rel1()
		u.mu.Lock()
		gotU1 := u.n["u1"]
		u.mu.Unlock()
		if gotU1 != 1 {
			t.Fatalf("after idempotent double release of rel1: n[u1] = %d, want 1", gotU1)
		}

		// Calling rel2 deletes the key at zero so map stays bounded.
		rel2()
		rel2()
		u.mu.Lock()
		_, stillPresent := u.n["u1"]
		u.mu.Unlock()
		if stillPresent {
			t.Fatal("u1 key still present in map after count reached zero")
		}

		relOther()
		u.mu.Lock()
		nLen := len(u.n)
		u.mu.Unlock()
		if nLen != 0 {
			t.Fatalf("map len = %d after all releases, want 0", nLen)
		}

		// Slot is usable again after release.
		rel3, ok := u.TryAcquire("u1")
		if !ok {
			t.Fatal("acquire u1 after full release failed")
		}
		rel3()
	})
}
