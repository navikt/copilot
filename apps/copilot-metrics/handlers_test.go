package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	healthHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"healthy"`) {
		t.Errorf("body = %q, want healthy status", body)
	}
}

func TestReadyHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	readyHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"ready"`) {
		t.Errorf("body = %q, want ready status", body)
	}
}

func TestIngestDay_DeleteError(t *testing.T) {
	ctx := t.Context()
	day := fixedDay()

	fetcher := &mockFetcher{
		result: &FetchResult{
			Records: singleRecord(),
			Scope:   "enterprise",
			ScopeID: "nav",
		},
	}
	store := &mockStore{
		dayExists: true,
		deleteErr: errTest,
	}

	err := ingestDay(ctx, fetcher, store, &Config{EnterpriseSlug: "nav"}, day)
	if err != nil {
		t.Fatalf("delete failure should not fail ingestion (entity skipped), got %v", err)
	}
	// Entity should NOT be re-inserted since delete failed
	if store.insertedCount != 0 {
		t.Errorf("expected no entity insert after delete failure, got %d", store.insertedCount)
	}
}

func TestIngestDay_InsertError(t *testing.T) {
	ctx := t.Context()
	day := fixedDay()

	fetcher := &mockFetcher{
		result: &FetchResult{
			Records: singleRecord(),
			Scope:   "enterprise",
			ScopeID: "nav",
		},
	}
	store := &mockStore{
		dayExists: false,
		insertErr: errTest,
	}

	err := ingestDay(ctx, fetcher, store, &Config{EnterpriseSlug: "nav"}, day)
	if err == nil {
		t.Fatal("expected error on insert failure, got nil")
	}
	if !strings.Contains(err.Error(), "failed to insert") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestIngestDay_DayExistsCheckError(t *testing.T) {
	ctx := t.Context()
	day := fixedDay()

	fetcher := &mockFetcher{
		result: &FetchResult{
			Records: singleRecord(),
			Scope:   "enterprise",
			ScopeID: "nav",
		},
	}
	store := &mockStore{
		dayExistsErr: errTest,
	}

	err := ingestDay(ctx, fetcher, store, &Config{EnterpriseSlug: "nav"}, day)
	if err == nil {
		t.Fatal("expected error on DayExists failure, got nil")
	}
	if !strings.Contains(err.Error(), "failed to check if day exists") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestIngestMissing_GetLatestDayError(t *testing.T) {
	ctx := context.Background()

	fetcher := &countingFetcher{
		result: &FetchResult{
			Records: []json.RawMessage{json.RawMessage(`{"test":"data"}`)},
			Scope:   "enterprise",
			ScopeID: "nav",
		},
	}
	store := &mockStore{
		latestDayErr: errors.New("BigQuery error"),
	}

	cfg := &Config{EnterpriseSlug: "nav"}
	err := ingestMissing(ctx, fetcher, store, cfg, nil)
	if err != nil {
		t.Fatalf("expected no error (fallback to yesterday), got %v", err)
	}
	if fetcher.calls != 1 {
		t.Errorf("expected 1 fetch call (yesterday only), got %d", fetcher.calls)
	}
}

func TestIngestMissing_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	threeDaysAgo := time.Now().UTC().AddDate(0, 0, -3)
	fetcher := &countingFetcher{
		result: &FetchResult{
			Records: singleRecord(),
			Scope:   "enterprise",
			ScopeID: "nav",
		},
	}
	store := &mockStore{latestDay: threeDaysAgo}
	cancel() // cancel immediately

	cfg := &Config{EnterpriseSlug: "nav"}
	err := ingestMissing(ctx, fetcher, store, cfg, nil)
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
}
