package types

import (
	"encoding/json"
)

type MetricsReportResponse struct {
	DownloadLinks []string `json:"download_links"`
	ReportDay     string   `json:"report_day"`
}

// FetchResult contains the metrics records along with metadata about the fetch.
type FetchResult struct {
	Records  []json.RawMessage
	Scope    string // "enterprise" or "organization"
	ScopeID  string // enterprise slug or org name
	Complete bool   // All required source endpoints returned validated reports.
}

// SeatCounts holds the Copilot seat totals for one scope. Totals only: it
// never carries per-person data.
type SeatCounts struct {
	Scope               string // "enterprise" or "organization"
	ScopeID             string // enterprise slug or org name
	Total               int
	PendingCancellation int
}
