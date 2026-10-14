package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeGH struct {
	teamErr error
	failed  map[string]bool // repos whose batch "failed" (nil result)
}

func (f *fakeGH) ListRepos(context.Context, string) ([]RepoInfo, error) {
	return []RepoInfo{{Name: "a"}, {Name: "b"}}, nil
}

func (f *fakeGH) BuildTeamMap(context.Context, string) (map[string][]TeamAccess, error) {
	return map[string][]TeamAccess{}, f.teamErr
}

func (f *fakeGH) ScanRepos(_ context.Context, _ string, repos []RepoInfo, criteria []SearchCriteria) (*ScanOutput, error) {
	out := &ScanOutput{Customizations: map[string]map[string]SearchResult{}, LastCommits: map[string]*time.Time{}}
	for _, r := range repos {
		if f.failed[r.Name] {
			out.Customizations[r.Name] = nil
		} else {
			out.Customizations[r.Name] = emptyResults(criteria)
		}
	}
	return out, nil
}

func (f *fakeGH) ResolveSourceOIDs(context.Context, []SearchCriteria) (SourceOIDs, error) {
	return SourceOIDs{}, nil
}

type fakeStore struct{ inserts int }

func (s *fakeStore) EnsureTableExists(context.Context) error { return nil }
func (s *fakeStore) EnsureViewsExist(context.Context) error  { return nil }
func (s *fakeStore) InsertScanResults(context.Context, time.Time, []RepoScanResult) error {
	s.inserts++
	return nil
}
func (s *fakeStore) DeleteScanDate(context.Context, time.Time) error         { return nil }
func (s *fakeStore) ScanDateExists(context.Context, time.Time) (bool, error) { return false, nil }
func (s *fakeStore) Close() error                                            { return nil }

func TestRunScan_FailsWithoutWritingOnTeamMapError(t *testing.T) {
	store := &fakeStore{}
	err := RunScan(context.Background(), &fakeGH{teamErr: errors.New("boom")}, store, &Config{OrganizationSlug: "navikt"}, time.Now(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if store.inserts != 0 {
		t.Errorf("wrote %d times, want 0", store.inserts)
	}
}

func TestRunScan_FailsWithoutWritingOnAnyFailedBatch(t *testing.T) {
	store := &fakeStore{}
	err := RunScan(context.Background(), &fakeGH{failed: map[string]bool{"b": true}}, store, &Config{OrganizationSlug: "navikt"}, time.Now(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if store.inserts != 0 {
		t.Errorf("wrote %d times, want 0", store.inserts)
	}
}

func TestRunScan_WritesWhenAllSucceed(t *testing.T) {
	store := &fakeStore{}
	if err := RunScan(context.Background(), &fakeGH{}, store, &Config{OrganizationSlug: "navikt"}, time.Now(), nil); err != nil {
		t.Fatal(err)
	}
	if store.inserts != 1 {
		t.Errorf("wrote %d times, want 1", store.inserts)
	}
}
