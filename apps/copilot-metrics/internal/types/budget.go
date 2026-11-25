package types

// BudgetEntry represents a single entry from the enterprise billing budgets API.
type BudgetEntry struct {
	BudgetScope      string   `json:"budget_scope"`
	BudgetEntityName string   `json:"budget_entity_name"`
	BudgetAmount     float64  `json:"budget_amount"`
	ConsumedAmount   *float64 `json:"consumed_amount"`
}

// UserBudgetData is the resolved effective budget and consumption for a single user.
type UserBudgetData struct {
	GitHubLogin    string
	BudgetAmount   float64
	ConsumedAmount float64
	IsOverride     bool
}
