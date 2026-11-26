package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestGlobalBudgetEnterpriseAICredits(t *testing.T) {
	amount := 5.0
	// A fresh cache stands in for the GitHub API.
	c := newBudgetClient(staticToken("token"), "nav")
	c.cachedAt = time.Now()
	c.cachedData = []BudgetEntry{
		{BudgetScope: "enterprise", BudgetProductSKU: "actions", BudgetAmount: 99},
		{BudgetScope: "enterprise", BudgetProductSKU: "ai_credits", BudgetAmount: 1234},
		{BudgetScope: "multi_user_customer", BudgetProductSKU: "ai_credits", BudgetAmount: 400},
		{BudgetScope: "user", BudgetProductSKU: "ai_credits", BudgetAmount: 400, ConsumedAmount: &amount},
	}

	b, err := c.getGlobalBudget(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if b.EnterpriseBudget != 1234 || b.PerUserBudget != 400 {
		t.Fatalf("got %+v", b)
	}
	raw, _ := json.Marshal(b)
	if string(raw) != `{"totalConsumed":5,"perUserBudget":400,"activeUsers":1}` {
		t.Fatalf("enterprise budget leaked into public JSON: %s", raw)
	}
}
