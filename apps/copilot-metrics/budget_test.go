package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
)

func TestBudgetClient_FetchUserEffectiveBudget(t *testing.T) {
	tests := []struct {
		name           string
		login          string
		responseStatus int
		responseBody   any
		wantBudget     float64
		wantConsumed   float64
		wantOverride   bool
		wantNil        bool
		wantErr        bool
	}{
		{
			name:           "standard user – enterprise default budget",
			login:          "paandahl",
			responseStatus: http.StatusOK,
			responseBody: map[string]any{
				"budgets": []map[string]any{
					{"budget_scope": "multi_user_customer", "budget_amount": 220},
				},
				"effective_budget": map[string]any{
					"id":              "eff-123",
					"budget_amount":   220.0,
					"consumed_amount": 60.37,
				},
			},
			wantBudget:   220.0,
			wantConsumed: 60.37,
			wantOverride: false,
		},
		{
			name:           "override user – individual budget",
			login:          "Starefossen",
			responseStatus: http.StatusOK,
			responseBody: map[string]any{
				"budgets": []map[string]any{
					{"budget_scope": "user", "budget_amount": 978},
				},
				"effective_budget": map[string]any{
					"id":              "eff-456",
					"budget_amount":   978.0,
					"consumed_amount": 120.41,
				},
			},
			wantBudget:   978.0,
			wantConsumed: 120.41,
			wantOverride: true,
		},
		{
			name:           "user not found returns nil",
			login:          "ghost",
			responseStatus: http.StatusNotFound,
			responseBody:   map[string]any{"message": "Not Found"},
			wantNil:        true,
		},
		{
			name:           "no effective_budget field returns nil",
			login:          "empty",
			responseStatus: http.StatusOK,
			responseBody: map[string]any{
				"budgets":          []any{},
				"effective_budget": nil,
			},
			wantNil: true,
		},
		{
			name:           "server error returns error",
			login:          "erruser",
			responseStatus: http.StatusInternalServerError,
			responseBody:   map[string]any{"message": "Internal Server Error"},
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test-billing-token" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if got := r.URL.Query().Get("user"); got != tt.login {
					t.Errorf("expected ?user=%s, got %s", tt.login, got)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.responseStatus)
				_ = json.NewEncoder(w).Encode(tt.responseBody)
			}))
			defer server.Close()

			client := &BudgetClient{
				httpClient: server.Client(),
				enterprise: "nav",
				token:      "test-billing-token",
			}
			// Point the client at our test server by replacing the URL in the request.
			// We override via a transport that rewrites the host.
			client.httpClient = &http.Client{
				Transport: &rewriteHostTransport{base: server.Client().Transport, target: server.URL},
			}

			got, err := client.FetchUserEffectiveBudget(context.Background(), tt.login)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil result")
			}
			if got.BudgetAmount != tt.wantBudget {
				t.Errorf("BudgetAmount = %v, want %v", got.BudgetAmount, tt.wantBudget)
			}
			if got.ConsumedAmount != tt.wantConsumed {
				t.Errorf("ConsumedAmount = %v, want %v", got.ConsumedAmount, tt.wantConsumed)
			}
			if got.IsOverride != tt.wantOverride {
				t.Errorf("IsOverride = %v, want %v", got.IsOverride, tt.wantOverride)
			}
			if got.GitHubLogin != tt.login {
				t.Errorf("GitHubLogin = %q, want %q", got.GitHubLogin, tt.login)
			}
		})
	}
}

func TestBudgetClient_FetchAllUserBudgets(t *testing.T) {
	logins := []string{"alice", "bob", "charlie"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		login := r.URL.Query().Get("user")
		amounts := map[string]float64{"alice": 10.0, "bob": 20.0, "charlie": 30.0}
		consumed, ok := amounts[login]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"budgets": []map[string]any{{"budget_scope": "multi_user_customer"}},
			"effective_budget": map[string]any{
				"budget_amount":   220.0,
				"consumed_amount": consumed,
			},
		})
	}))
	defer server.Close()

	client := &BudgetClient{
		httpClient: &http.Client{
			Transport: &rewriteHostTransport{base: server.Client().Transport, target: server.URL},
		},
		enterprise: "nav",
		token:      "",
	}

	results, err := client.FetchAllUserBudgets(context.Background(), logins)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	sort.Slice(results, func(i, j int) bool { return results[i].GitHubLogin < results[j].GitHubLogin })
	for i, want := range []struct {
		login    string
		consumed float64
	}{{"alice", 10.0}, {"bob", 20.0}, {"charlie", 30.0}} {
		if results[i].GitHubLogin != want.login {
			t.Errorf("[%d] login = %q, want %q", i, results[i].GitHubLogin, want.login)
		}
		if results[i].ConsumedAmount != want.consumed {
			t.Errorf("[%d] consumed = %v, want %v", i, results[i].ConsumedAmount, want.consumed)
		}
	}
}

func TestBudgetClient_FetchAllUserBudgets_PartialFailure(t *testing.T) {
	logins := []string{"ok1", "notfound", "ok2"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		login := r.URL.Query().Get("user")
		if login == "notfound" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"budgets": []map[string]any{{"budget_scope": "multi_user_customer"}},
			"effective_budget": map[string]any{
				"budget_amount":   220.0,
				"consumed_amount": 5.0,
			},
		})
	}))
	defer server.Close()

	client := &BudgetClient{
		httpClient: &http.Client{
			Transport: &rewriteHostTransport{base: server.Client().Transport, target: server.URL},
		},
		enterprise: "nav",
		token:      "",
	}

	// notfound returns nil (not error) — so all 3 succeed, just 2 have data
	results, err := client.FetchAllUserBudgets(context.Background(), logins)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results (notfound skipped), got %d", len(results))
	}
}

func TestGitHubClient_FetchAllCopilotLogins(t *testing.T) {
	// Two pages: first has 2 seats, second has 1 seat
	page := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !pathContains(r.URL.Path, "copilot/billing/seats") {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		page++
		w.Header().Set("Content-Type", "application/json")
		switch page {
		case 1:
			_ = json.NewEncoder(w).Encode(copilotSeatResponse{
				TotalSeats: 3,
				Seats: []struct {
					Assignee struct {
						Login string `json:"login"`
					} `json:"assignee"`
				}{
					{Assignee: struct {
						Login string `json:"login"`
					}{Login: "alice"}},
					{Assignee: struct {
						Login string `json:"login"`
					}{Login: "bob"}},
				},
			})
		case 2:
			_ = json.NewEncoder(w).Encode(copilotSeatResponse{
				TotalSeats: 3,
				Seats: []struct {
					Assignee struct {
						Login string `json:"login"`
					} `json:"assignee"`
				}{
					{Assignee: struct {
						Login string `json:"login"`
					}{Login: "charlie"}},
				},
			})
		default:
			// Should not be reached — return empty to stop pagination
			_ = json.NewEncoder(w).Encode(copilotSeatResponse{TotalSeats: 3, Seats: nil})
		}
	}))
	defer server.Close()

	client := &GitHubClient{
		httpClient: &http.Client{
			Transport: &rewriteHostTransport{base: server.Client().Transport, target: server.URL},
		},
		enterprise: "nav",
		org:        "navikt",
	}

	logins, err := client.FetchAllCopilotLogins(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(logins) != 3 {
		t.Fatalf("expected 3 logins, got %d: %v", len(logins), logins)
	}
	want := []string{"alice", "bob", "charlie"}
	for i, w := range want {
		if logins[i] != w {
			t.Errorf("logins[%d] = %q, want %q", i, logins[i], w)
		}
	}
}

// rewriteHostTransport rewrites outgoing requests to hit a test server instead of GitHub.
type rewriteHostTransport struct {
	base   http.RoundTripper
	target string
}

func (t *rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = "http"
	clone.URL.Host = t.target[len("http://"):]
	if t.base != nil {
		return t.base.RoundTrip(clone)
	}
	return http.DefaultTransport.RoundTrip(clone)
}

func pathContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
