package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBillingClient_FetchMonthlyUsage(t *testing.T) {
	response := BillingUsageResponse{
		TimePeriod: struct {
			Year  int `json:"year"`
			Month int `json:"month,omitempty"`
			Day   int `json:"day,omitempty"`
		}{Year: 2026, Month: 5},
		Enterprise: "nav",
		UsageItems: []BillingUsageItem{
			{
				Product:       "Copilot",
				SKU:           "Copilot Premium Request",
				Model:         "Claude Opus 4.7",
				UnitType:      "requests",
				PricePerUnit:  0.04,
				GrossQuantity: 100.0,
				GrossAmount:   4.0,
				NetQuantity:   80.0,
				NetAmount:     3.2,
			},
			{
				Product:       "Copilot",
				SKU:           "Copilot Premium Request",
				Model:         "GPT-5.5",
				UnitType:      "requests",
				PricePerUnit:  0.04,
				GrossQuantity: 50.0,
				GrossAmount:   2.0,
				NetQuantity:   40.0,
				NetAmount:     1.6,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %s", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.Header.Get("X-GitHub-Api-Version") != "2026-03-10" {
			t.Errorf("expected API version 2026-03-10, got %s", r.Header.Get("X-GitHub-Api-Version"))
		}

		query := r.URL.Query()
		if query.Get("year") != "2026" {
			t.Errorf("expected year=2026, got %s", query.Get("year"))
		}
		if query.Get("month") != "5" {
			t.Errorf("expected month=5, got %s", query.Get("month"))
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := &BillingClient{
		httpClient: server.Client(),
		enterprise: "nav",
		token:      "test-token",
	}
	if client.token != "test-token" {
		t.Errorf("expected token 'test-token', got %s", client.token)
	}

	nilClient := NewBillingClient("", "nav")
	if nilClient != nil {
		t.Error("expected nil client with empty token")
	}

	validClient := NewBillingClient("some-token", "nav")
	if validClient == nil {
		t.Fatal("expected non-nil client with valid token")
	}
	if validClient.enterprise != "nav" {
		t.Errorf("expected enterprise 'nav', got %s", validClient.enterprise)
	}
}

func TestBillingUsageItem_Marshaling(t *testing.T) {
	item := BillingUsageItem{
		Product:       "Copilot",
		SKU:           "Copilot Premium Request",
		Model:         "Claude Opus 4.7",
		UnitType:      "requests",
		PricePerUnit:  0.04,
		GrossQuantity: 163672.33,
		GrossAmount:   6546.89,
		NetQuantity:   138331.66,
		NetAmount:     5533.27,
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded BillingUsageItem
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Model != "Claude Opus 4.7" {
		t.Errorf("expected model 'Claude Opus 4.7', got %s", decoded.Model)
	}
	if decoded.GrossQuantity != 163672.33 {
		t.Errorf("expected gross quantity 163672.33, got %f", decoded.GrossQuantity)
	}
}

func TestBillingClient_FetchOrganizationUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/orgs/navikt/settings/billing/usage"; got != want {
			t.Fatalf("unexpected path: got %s want %s", got, want)
		}
		query := r.URL.Query()
		if query.Get("year") != "2026" || query.Get("month") != "6" || query.Get("day") != "7" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("missing auth header")
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(OrganizationBillingUsageResponse{
			UsageItems: []OrganizationBillingUsageItem{
				{
					Date:             "2026-06-07",
					Product:          "Copilot",
					SKU:              "Copilot Premium Request",
					Quantity:         10,
					UnitType:         "requests",
					PricePerUnit:     0.04,
					GrossAmount:      0.4,
					DiscountAmount:   0,
					NetAmount:        0.4,
					OrganizationName: "navikt",
					RepositoryName:   "navikt/copilot",
				},
			},
		})
	}))
	defer server.Close()

	client := &BillingClient{
		httpClient: server.Client(),
		token:      "test-token",
	}
	day := time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC)

	client.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = server.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(req)
	})

	resp, err := client.FetchOrganizationUsage(context.Background(), "navikt", day)
	if err != nil {
		t.Fatalf("FetchOrganizationUsage error: %v", err)
	}
	if len(resp.UsageItems) != 1 {
		t.Fatalf("expected 1 usage item, got %d", len(resp.UsageItems))
	}
	if resp.UsageItems[0].RepositoryName != "navikt/copilot" {
		t.Fatalf("unexpected repository_name: %s", resp.UsageItems[0].RepositoryName)
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
