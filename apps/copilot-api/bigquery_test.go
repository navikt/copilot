package main

import (
	"context"
	"testing"
	"time"
)

// A login in another case must hit the same cache entry, not start a new scan.
// The client is nil, so a cache miss would panic.
func TestCachedUserMetricsKeyIgnoresCase(t *testing.T) {
	c := newCachedBigQueryClient(nil, time.Minute)
	want := &UserMetricsSummary{}
	c.cache.Set("user_metrics_octocat_7", want)
	got, err := c.GetUserMetrics(context.Background(), "OctoCat", 7)
	if err != nil || got != want {
		t.Fatalf("GetUserMetrics(OctoCat) = %v, %v; want the cached octocat entry", got, err)
	}
}
