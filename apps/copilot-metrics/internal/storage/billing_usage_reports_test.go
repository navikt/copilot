package storage

import (
	"testing"
)

func TestNullableRepositoryName_EmptyIsNull(t *testing.T) {
	got := nullableRepositoryName("")
	if got.Valid {
		t.Fatalf("expected invalid null string for empty repository name")
	}
}

func TestNullableRepositoryName_ValueIsSet(t *testing.T) {
	got := nullableRepositoryName("navikt/copilot")
	if !got.Valid {
		t.Fatalf("expected valid null string for repository name")
	}
	if got.StringVal != "navikt/copilot" {
		t.Fatalf("unexpected repository name: %s", got.StringVal)
	}
}
