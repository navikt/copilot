package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
		// Verify auth header
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %s", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Verify API version header
		if r.Header.Get("X-GitHub-Api-Version") != "2026-03-10" {
			t.Errorf("expected API version 2026-03-10, got %s", r.Header.Get("X-GitHub-Api-Version"))
		}

		// Verify query params
		query := r.URL.Query()
		if query.Get("year") != "2026" {
			t.Errorf("expected year=2026, got %s", query.Get("year"))
		}
		if query.Get("month") != "5" {
			t.Errorf("expected month=5, got %s", query.Get("month"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client pointing at test server
	client := &BillingClient{
		httpClient: server.Client(),
		enterprise: "nav",
		token:      "test-token",
	}
	if client.token != "test-token" {
		t.Errorf("expected token 'test-token', got %s", client.token)
	}

	// Test NewBillingClient with empty token returns nil
	nilClient := NewBillingClient("", "nav")
	if nilClient != nil {
		t.Error("expected nil client with empty token")
	}

	// Test NewBillingClient with token returns non-nil
	validClient := NewBillingClient("some-token", "nav")
	if validClient == nil {
		t.Error("expected non-nil client with valid token")
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
