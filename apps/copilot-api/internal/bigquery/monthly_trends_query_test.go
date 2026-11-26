package bigquery

import (
	"os"
	"strings"
	"testing"
)

// Credits per user and cohort retention are Nav-wide and name no one, so their
// queries carry no minimum group size.
func TestAnonymousTrendQueriesHaveNoMinimum(t *testing.T) {
	b, err := os.ReadFile("monthly_trends_query.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "@min_users") {
		t.Error("monthly_trends_query.go still filters on @min_users")
	}
}
