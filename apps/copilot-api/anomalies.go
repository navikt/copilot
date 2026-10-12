package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/googleapi"
)

// Start thresholds from PRD 4.3 (decision S10). The owner tunes them after
// three months of use, so they live here and nowhere else.
const (
	spikeFactor       = 2.0  // A1: day above 2 × median for the same day type
	spikeMinUSD       = 100  // A1: and at least this far above the median
	spikeBaselineDays = 28   // A1: median over the previous 28 days
	newModelMinUSD    = 10.0 // A3: first day a model costs more than this
	staleDataDays     = 2    // A7: no new gross rows for this many days
	netRunDueDay      = 5    // A7: last month's net must be complete by this day
	minBaselineDays   = 4    // A1: skip days with a thinner baseline
	// A1: gross per day roughly tripled when billing moved from premium requests
	// to AI credits. Days before the switch are no baseline for days after it.
	billingBasisSwitch = "2026-06-01"
	anomalyWindowDays  = 30 // anomalies shown for the last 30 days
	anomalyLookbackDay = anomalyWindowDays + spikeBaselineDays
)

// Anomaly is one finding for /innsikt/kostnad. Aggregates only: no people.
type Anomaly struct {
	Rule      string  `json:"rule"`
	Date      string  `json:"date"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	Model     string  `json:"model,omitempty"`
}

type anomalyDay struct {
	Day   string  `bigquery:"day"`
	Gross float64 `bigquery:"gross"`
}

type anomalyModel struct {
	Model    string  `bigquery:"model"`
	FirstDay string  `bigquery:"first_day"`
	Gross    float64 `bigquery:"gross"`
}

type anomalyInput struct {
	Days         []anomalyDay   // gross per day, ascending, from the lookback start
	NewModels    []anomalyModel // first day each model cost > newModelMinUSD
	FirstDataDay string         // first day in billing_usage_daily_model
	NetComplete  bool           // last month has a complete billing run
}

// findAnomalies applies A7, A1 and A3 to the input, A7 first (K12).
// today is the UTC date the page is viewed.
func findAnomalies(in anomalyInput, today time.Time) []Anomaly {
	out := []Anomaly{}
	day := func(s string) time.Time { t, _ := time.Parse(time.DateOnly, s); return t }
	from := today.AddDate(0, 0, -anomalyWindowDays)

	// A7: stale data, gaps inside the window, and missing net for last month.
	if len(in.Days) == 0 || today.Sub(day(in.Days[len(in.Days)-1].Day)) > staleDataDays*24*time.Hour {
		last := ""
		if len(in.Days) > 0 {
			last = in.Days[len(in.Days)-1].Day
		}
		out = append(out, Anomaly{Rule: "A7", Date: last, Threshold: staleDataDays})
	}
	// Walk expected days from the later of the window start and the first data
	// day, so a gap that began before the query window is still seen.
	next := from
	if first := day(in.FirstDataDay); first.After(next) {
		next = first
	}
	for _, x := range in.Days {
		for ; next.Before(day(x.Day)); next = next.AddDate(0, 0, 1) {
			out = append(out, Anomaly{Rule: "A7", Date: next.Format(time.DateOnly)})
		}
		if !next.After(day(x.Day)) {
			next = day(x.Day).AddDate(0, 0, 1)
		}
	}
	if today.Day() >= netRunDueDay && !in.NetComplete && in.FirstDataDay != "" && in.FirstDataDay < time.Date(today.Year(), today.Month()-1, 1, 0, 0, 0, 0, time.UTC).Format(time.DateOnly) {
		prev := time.Date(today.Year(), today.Month()-1, 1, 0, 0, 0, 0, time.UTC)
		out = append(out, Anomaly{Rule: "A7", Date: prev.Format("2006-01"), Threshold: netRunDueDay})
	}

	// A1: spike against the median of the same day type in the last 28 days.
	weekend := func(t time.Time) bool { return t.Weekday() == time.Saturday || t.Weekday() == time.Sunday }
	for i, d := range in.Days {
		t := day(d.Day)
		if t.Before(from) {
			continue
		}
		var base []float64
		for _, p := range in.Days[:i] {
			pt := day(p.Day)
			if t.Sub(pt) <= spikeBaselineDays*24*time.Hour && weekend(pt) == weekend(t) && (d.Day < billingBasisSwitch || p.Day >= billingBasisSwitch) {
				base = append(base, p.Gross)
			}
		}
		if len(base) < minBaselineDays {
			continue
		}
		m := median(base)
		if d.Gross > spikeFactor*m && d.Gross-m >= spikeMinUSD {
			out = append(out, Anomaly{Rule: "A1", Date: d.Day, Value: d.Gross, Threshold: m})
		}
	}

	// A3: a model's first costly day. Skip the first 28 days of data, where
	// every model looks new.
	known := day(in.FirstDataDay).AddDate(0, 0, spikeBaselineDays)
	for _, m := range in.NewModels {
		t := day(m.FirstDay)
		if !t.Before(from) && !t.Before(known) {
			out = append(out, Anomaly{Rule: "A3", Date: m.FirstDay, Model: m.Model, Value: m.Gross, Threshold: newModelMinUSD})
		}
	}
	return out
}

func median(v []float64) float64 {
	s := slices.Clone(v)
	slices.Sort(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// GetCostAnomalies reads the inputs for findAnomalies. It computes on request
// and stores nothing (K11).
func (bq *BigQueryClient) GetCostAnomalies(ctx context.Context) ([]Anomaly, error) {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	daily := bq.tableRef(bq.metricsDataset, "billing_usage_daily_model")
	from := today.AddDate(0, 0, -anomalyLookbackDay).Format(time.DateOnly)
	prev := time.Date(today.Year(), today.Month()-1, 1, 0, 0, 0, 0, time.UTC).Format(time.DateOnly)

	var in anomalyInput
	q := bq.client.Query(fmt.Sprintf(`SELECT FORMAT_DATE('%%F',day) day,SUM(gross_amount) gross FROM %s
WHERE LOWER(scope_id)='nav' AND product='Copilot' AND gross_quantity>0 AND day>=DATE(@from) GROUP BY day ORDER BY day`, daily))
	q.Parameters = []bigquery.QueryParameter{{Name: "from", Value: from}}
	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read daily gross: %w", err)
	}
	if in.Days, err = readAllRows[anomalyDay](it); err != nil {
		return nil, err
	}

	q = bq.client.Query(fmt.Sprintf(`WITH m AS (
 SELECT day,REGEXP_REPLACE(model,r'^Auto: ','') model,SUM(gross_amount) gross FROM %s WHERE LOWER(scope_id)='nav' AND product='Copilot' AND gross_quantity>0 GROUP BY 1,2
) SELECT model,FORMAT_DATE('%%F',MIN(day)) first_day,ARRAY_AGG(gross ORDER BY day LIMIT 1)[OFFSET(0)] gross
FROM m WHERE gross>@minUSD GROUP BY model HAVING MIN(day)>=DATE(@from)`, daily))
	q.Parameters = []bigquery.QueryParameter{{Name: "from", Value: from}, {Name: "minUSD", Value: newModelMinUSD}}
	if it, err = q.Read(ctx); err != nil {
		return nil, fmt.Errorf("read new models: %w", err)
	}
	if in.NewModels, err = readAllRows[anomalyModel](it); err != nil {
		return nil, err
	}

	// The runs table only exists after the first backfill; a missing table
	// counts as no complete run, which is what A7 should say.
	type meta struct {
		FirstDay string `bigquery:"first_day"`
		Complete bool   `bigquery:"complete"`
	}
	runs := "FALSE"
	if _, err := bq.client.Dataset(bq.metricsDataset).Table("billing_user_monthly_runs").Metadata(ctx); err != nil {
		var apiErr *googleapi.Error
		if !errors.As(err, &apiErr) || apiErr.Code != 404 {
			return nil, fmt.Errorf("read billing run metadata: %w", err)
		}
	} else {
		runs = fmt.Sprintf("(SELECT COUNT(*)>0 FROM %s WHERE month=DATE(@prev) AND scope_id='nav' AND status='complete')",
			bq.tableRef(bq.metricsDataset, "billing_user_monthly_runs"))
	}
	q = bq.client.Query(fmt.Sprintf(`SELECT IFNULL(FORMAT_DATE('%%F',MIN(day)),'') first_day,%s complete FROM %s WHERE LOWER(scope_id)='nav' AND product='Copilot'`, runs, daily))
	q.Parameters = []bigquery.QueryParameter{{Name: "prev", Value: prev}}
	if it, err = q.Read(ctx); err != nil {
		return nil, fmt.Errorf("read billing coverage: %w", err)
	}
	rows, err := readAllRows[meta](it)
	if err != nil {
		return nil, err
	}
	if len(rows) == 1 {
		in.FirstDataDay, in.NetComplete = rows[0].FirstDay, rows[0].Complete
	}
	return findAnomalies(in, today), nil
}

func (c *CachedBigQueryClient) GetCostAnomalies(ctx context.Context) ([]Anomaly, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	return getCachedValue(c, "cost_anomalies:"+time.Now().UTC().Format(time.DateOnly), func() ([]Anomaly, error) {
		return c.client.GetCostAnomalies(ctx)
	})
}

// handleCostAnomalies must sit behind requireAdmin: costs are for admins only.
func (h *BigQueryHandlers) handleCostAnomalies(w http.ResponseWriter, r *http.Request) {
	anomalies, err := h.bqClient.GetCostAnomalies(r.Context())
	if err != nil {
		slog.Error("Failed to compute cost anomalies", "error", err)
		respondError(w, "internal_error", "Failed to compute cost anomalies", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 900, false)
	respondJSON(w, anomalies, http.StatusOK)
}
