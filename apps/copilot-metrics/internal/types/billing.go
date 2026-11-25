package types

// BillingUsageResponse is the response from the premium request usage endpoint.
type BillingUsageResponse struct {
	TimePeriod struct {
		Year  int `json:"year"`
		Month int `json:"month,omitempty"`
		Day   int `json:"day,omitempty"`
	} `json:"timePeriod"`
	Enterprise string             `json:"enterprise"`
	User       string             `json:"user,omitempty"`
	UsageItems []BillingUsageItem `json:"usageItems"`
}

// OrganizationBillingUsageResponse is the response from the organization billing usage report endpoint.
type OrganizationBillingUsageResponse struct {
	UsageItems []OrganizationBillingUsageItem `json:"usageItems"`
}

// OrganizationBillingUsageItem represents a single organization billing usage line item.
type OrganizationBillingUsageItem struct {
	Date             string  `json:"date"`
	Product          string  `json:"product"`
	SKU              string  `json:"sku"`
	Quantity         float64 `json:"quantity"`
	UnitType         string  `json:"unitType"`
	PricePerUnit     float64 `json:"pricePerUnit"`
	GrossAmount      float64 `json:"grossAmount"`
	DiscountAmount   float64 `json:"discountAmount"`
	NetAmount        float64 `json:"netAmount"`
	OrganizationName string  `json:"organizationName"`
	RepositoryName   string  `json:"repositoryName,omitempty"`
}

// BillingUsageItem represents a single line item from the billing API.
type BillingUsageItem struct {
	Product          string  `json:"product"`
	SKU              string  `json:"sku"`
	Model            string  `json:"model"`
	UnitType         string  `json:"unitType"`
	PricePerUnit     float64 `json:"pricePerUnit"`
	GrossQuantity    float64 `json:"grossQuantity"`
	GrossAmount      float64 `json:"grossAmount"`
	DiscountQuantity float64 `json:"discountQuantity"`
	DiscountAmount   float64 `json:"discountAmount"`
	NetQuantity      float64 `json:"netQuantity"`
	NetAmount        float64 `json:"netAmount"`
}
