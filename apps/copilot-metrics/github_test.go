package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDownloadAndParseNDJSON(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		status      int
		wantRecords int
		wantErr     bool
	}{
		{
			name:        "single record",
			body:        `{"day":"2025-10-11","daily_active_users":30}`,
			status:      http.StatusOK,
			wantRecords: 1,
		},
		{
			name:        "multiple records",
			body:        "{\"a\":1}\n{\"b\":2}\n{\"c\":3}\n",
			status:      http.StatusOK,
			wantRecords: 3,
		},
		{
			name:        "blank lines skipped",
			body:        "{\"a\":1}\n\n{\"b\":2}\n\n",
			status:      http.StatusOK,
			wantRecords: 2,
		},
		{
			name:        "invalid JSON lines skipped",
			body:        "{\"a\":1}\nnot json\n{\"b\":2}\n",
			status:      http.StatusOK,
			wantRecords: 2,
		},
		{
			name:        "empty body",
			body:        "",
			status:      http.StatusOK,
			wantRecords: 0,
		},
		{
			name:    "non-200 status",
			body:    "forbidden",
			status:  http.StatusForbidden,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "" {
					t.Error("download request should NOT have Authorization header")
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := &GitHubClient{
				httpClient:     &http.Client{},
				downloadClient: server.Client(),
			}

			records, err := client.downloadAndParseNDJSON(context.Background(), server.URL)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(records) != tt.wantRecords {
				t.Errorf("got %d records, want %d", len(records), tt.wantRecords)
			}
		})
	}
}

func TestDownloadAndParseNDJSON_LargeRecord(t *testing.T) {
	// Simulate a large NDJSON record like the real enterprise metrics
	largeValue := strings.Repeat("x", 500_000)
	body := `{"data":"` + largeValue + `"}` + "\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	client := &GitHubClient{
		downloadClient: server.Client(),
	}

	records, err := client.downloadAndParseNDJSON(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("got %d records, want 1", len(records))
	}
}

func TestDownloadUsesDownloadClient(t *testing.T) {
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("httpClient (auth) should NOT be used for downloads")
		w.WriteHeader(http.StatusOK)
	}))
	defer authServer.Close()

	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}` + "\n"))
	}))
	defer downloadServer.Close()

	client := &GitHubClient{
		httpClient:     authServer.Client(),
		downloadClient: downloadServer.Client(),
	}

	records, err := client.downloadAndParseNDJSON(context.Background(), downloadServer.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("got %d records, want 1", len(records))
	}
}
