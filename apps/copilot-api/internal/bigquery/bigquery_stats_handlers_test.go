package bigquery

import "testing"

func TestIsValidYearMonth(t *testing.T) {
	tests := []struct {
		name  string
		month string
		want  bool
	}{
		{"valid month", "2026-06", true},
		{"valid january", "2026-01", true},
		{"valid december", "2026-12", true},
		{"invalid separator", "2026/06", false},
		{"invalid calendar month", "2026-13", false},
		{"invalid zero month", "2026-00", false},
		{"empty string", "", false},
		{"missing day would be wrong format anyway", "2026-06-01", false},
		{"garbage", "not-a-month", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsValidYearMonth(tc.month); got != tc.want {
				t.Errorf("IsValidYearMonth(%q) = %v, want %v", tc.month, got, tc.want)
			}
		})
	}
}

func TestSuppressSmallBuckets(t *testing.T) {
	buckets := []UsageHistogramBucket{
		{Bucket: "0%", NumUsers: 0},
		{Bucket: "1-9%", NumUsers: 1},
		{Bucket: "10-24%", NumUsers: minUsersForDistribution - 1},
		{Bucket: "25-49%", NumUsers: minUsersForDistribution},
	}
	suppressSmallBuckets(buckets)

	want := []UsageHistogramBucket{
		{Bucket: "0%", NumUsers: 0},
		{Bucket: "1-9%", NumUsers: 0, Suppressed: true},
		{Bucket: "10-24%", NumUsers: 0, Suppressed: true},
		{Bucket: "25-49%", NumUsers: minUsersForDistribution},
	}
	for i := range want {
		if buckets[i] != want[i] {
			t.Errorf("bucket %d = %+v, want %+v", i, buckets[i], want[i])
		}
	}
}
