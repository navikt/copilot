package main

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestDownloadPreservesUsageQualityFields(t *testing.T) {
	// Synthetic documented fields. The unpublished requirements object is not guessed.
	for _, raw := range []string{
		`{"day":"2026-11-01"}`,
		`{"day":"2026-11-03","missing_client_telemetry_user_count":0,"below_usage_metrics_minimum_user_count":6}`,
		`{"day":"2026-11-05","client_telemetry_present":false,"usage_metrics_requirements":null}`,
		`{"day":"2026-11-05","client_telemetry_present":true}`,
		`{"day":"2026-11-05","client_telemetry_present":null,"missing_client_telemetry_user_count":null}`,
	} {
		client := &GitHubClient{downloadClient: mockClient(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(raw + "\n"))
		}))}
		records, err := client.downloadAndParseNDJSON(context.Background(), "http://test/download")
		if err != nil {
			t.Fatal(err)
		}
		if len(records) != 1 || strings.TrimSpace(string(records[0])) != raw {
			t.Fatalf("raw report changed: %s", records)
		}
	}
}
