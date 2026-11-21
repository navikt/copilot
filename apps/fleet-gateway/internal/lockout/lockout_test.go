package lockout

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/navikt/copilot/fleet-gateway/internal/session"
)

func newTestKey() []byte {
	// Generated per call so no key-shaped literal lives in the source.
	return []byte(rand.Text() + rand.Text())
}

func TestUserKey_CaseInsensitiveAndTrimmed(t *testing.T) {
	key := newTestKey()

	want := UserKey(key, "A123456")
	if want == "" {
		t.Fatal("UserKey returned empty string")
	}

	variants := []string{
		"a123456",
		"A123456",
		" A123456 ",
		"\ta123456\n",
	}
	for _, ident := range variants {
		if got := UserKey(key, ident); got != want {
			t.Errorf("UserKey(key, %q) = %q, want %q", ident, got, want)
		}
	}

	if other := UserKey(key, "B123456"); other == want {
		t.Errorf("UserKey(key, %q) collided with %q", "B123456", "A123456")
	}
}

func TestManager_LockoutCaseInsensitiveAndTrimmed(t *testing.T) {
	variants := []string{"a123456", "A123456", " A123456 "}

	for _, lockedIdent := range variants {
		t.Run("lock_"+lockedIdent, func(t *testing.T) {
			ctx := context.Background()
			now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
			clock := func() time.Time { return now }

			store := session.NewMemoryStore(clock, nil)
			t.Cleanup(store.Close)
			mgr := New(store, newTestKey(), nil, clock)

			if _, err := mgr.LockOutUser(ctx, lockedIdent, "op1"); err != nil {
				t.Fatalf("LockOutUser(%q): %v", lockedIdent, err)
			}

			afterLockout := now.Add(time.Minute)
			for _, checkIdent := range variants {
				revoked, err := mgr.UserRevoked(ctx, checkIdent, afterLockout)
				if err != nil {
					t.Fatalf("UserRevoked(%q): %v", checkIdent, err)
				}
				if !revoked {
					t.Errorf("UserRevoked(%q) = false after locking out %q, want true", checkIdent, lockedIdent)
				}
			}

			otherRevoked, err := mgr.UserRevoked(ctx, "B123456", afterLockout)
			if err != nil {
				t.Fatalf("UserRevoked(%q): %v", "B123456", err)
			}
			if otherRevoked {
				t.Errorf("UserRevoked(%q) = true after locking out %q, want false", "B123456", lockedIdent)
			}

			// Undoing with a differently cased variant lifts the active block for all variants.
			undoIdent := "a123456"
			if lockedIdent == "a123456" {
				undoIdent = " A123456 "
			}
			if err := mgr.UndoLockout(ctx, undoIdent, "op1"); err != nil {
				t.Fatalf("UndoLockout(%q): %v", undoIdent, err)
			}
			for _, checkIdent := range variants {
				revoked, err := mgr.UserRevoked(ctx, checkIdent, afterLockout)
				if err != nil {
					t.Fatalf("UserRevoked(%q) after undo: %v", checkIdent, err)
				}
				if revoked {
					t.Errorf("UserRevoked(%q) = true after UndoLockout(%q), want false", checkIdent, undoIdent)
				}
			}
		})
	}
}
