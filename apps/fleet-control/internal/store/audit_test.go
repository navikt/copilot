package store

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestAuditTracerForget(t *testing.T) {
	c := &pgx.Conn{}
	PendingAudit.add(c, loggedAudit{})
	PendingAudit.Forget(c)
	PendingAudit.mu.Lock()
	defer PendingAudit.mu.Unlock()
	if _, ok := PendingAudit.pending[c]; ok {
		t.Fatal("rows kept for a closed connection")
	}
}
