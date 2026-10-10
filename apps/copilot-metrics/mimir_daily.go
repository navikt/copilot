package main

// Daily copy of Mimir aggregates to BigQuery, so history outlives Mimir's
// ~70-day retention. A Go port of copilot-intern underlag/mimir-backfill/
// backfill.py: same families, grain, k>=5 device threshold and columns.
//
// Device ids are read only to count distinct devices per row; they are never
// written. Rows with fewer than five devices get their model or tool merged
// into "andre"; an "andre" row still under five is dropped.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
)

const (
	mimirK         = 5
	mimirDevice    = "nav_pilot_device_id"
	mimirModel     = "gen_ai_request_model"
	mimirDailyName = "mimir_daily"
)

type mimirCol struct{ name, label string }

type mimirSpec struct {
	family, metric string
	cols           []mimirCol // first is always job
	merge          string     // column merged into "andre"
	hist, reliable bool
}

func jobModel() []mimirCol { return []mimirCol{{"job", "job"}, {"model", mimirModel}} }

// Copilot sessions running concurrently share one series, so Prometheus reads
// every drop as a counter reset and increase() overcounts. Token counters are
// known to be hit; tool-call and code-lines counters come from the same
// exporter and are flagged too.
var mimirSpecs = func() []mimirSpec {
	s := []mimirSpec{{"token_usage", "gen_ai_client_token_usage", append(jobModel(), mimirCol{"token_type", "gen_ai_token_type"}), "model", true, false}}
	for _, t := range []string{"input", "output", "cache_read_input", "cache_write_input", "reasoning_output"} {
		s = append(s, mimirSpec{"inference_tokens", "gen_ai_client_inference_usage_" + t + "_tokens_total", jobModel(), "model", false, false})
	}
	tool := []mimirCol{{"job", "job"}, {"tool", "gen_ai_tool_name"}}
	return append(s,
		mimirSpec{"operation_duration", "gen_ai_client_operation_duration_seconds", append(jobModel(), mimirCol{"operation", "gen_ai_operation_name"}, mimirCol{"error_type", "error_type"}), "model", true, true},
		mimirSpec{"execute_tool", "gen_ai_execute_tool_duration_seconds", append(tool, mimirCol{"error_type", "error_type"}), "tool", true, true},
		mimirSpec{"tool_call", "github_copilot_tool_call_duration_seconds", tool, "tool", true, true},
		mimirSpec{"tool_call", "github_copilot_tool_call_count_total", append(tool, mimirCol{"success", "success"}), "tool", false, false},
		mimirSpec{"code_lines", "github_copilot_code_lines_added_total", jobModel(), "model", false, false},
		mimirSpec{"code_lines", "github_copilot_code_lines_removed_total", jobModel(), "model", false, false},
	)
}()

// Nav-wide copilot-chat series carry no device label: no threshold possible.
var mimirChat = []mimirSpec{
	{metric: "copilot_chat_edit_acceptance_count_total", cols: []mimirCol{{"edit_outcome", "copilot_chat_edit_outcome"}, {"edit_source", "copilot_chat_edit_source"}}},
	{metric: "copilot_chat_chat_edit_outcome_count_total"},
	{metric: "copilot_chat_lines_of_code_count_total"},
	{metric: "copilot_chat_edit_survival_four_gram", cols: []mimirCol{{"edit_source", "copilot_chat_edit_source"}, {"time_delay_ms", "copilot_chat_time_delay_ms"}}, hist: true},
	{metric: "copilot_chat_edit_survival_no_revert", cols: []mimirCol{{"edit_source", "copilot_chat_edit_source"}, {"time_delay_ms", "copilot_chat_time_delay_ms"}}, hist: true},
}

type MimirClient struct {
	BaseURL string // .../prometheus/api/v1
	HTTP    *http.Client
}

type promSample struct {
	Metric map[string]string `json:"metric"`
	Value  [2]any            `json:"value"`
}

func (m *MimirClient) query(ctx context.Context, expr string, t time.Time) ([]promSample, error) {
	u := m.BaseURL + "/query?" + url.Values{"query": {expr}, "time": {t.Format(time.RFC3339)}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Scope-OrgID", "nais")
	resp, err := m.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var d struct {
		Status string `json:"status"`
		Error  string `json:"error"`
		Data   struct {
			Result []promSample `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(&d); err != nil {
		return nil, fmt.Errorf("mimir %d: %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK || d.Status != "success" {
		return nil, fmt.Errorf("mimir %d: %s: %s", resp.StatusCode, d.Status, d.Error)
	}
	return d.Data.Result, nil
}

func (s promSample) float() float64 {
	str, _ := s.Value[1].(string)
	f, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return math.NaN()
	}
	return f
}

func labelsOf(cols []mimirCol) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.label
	}
	return out
}

func keyOf(s promSample, labels []string) []string {
	k := make([]string, len(labels))
	for i, l := range labels {
		k[i] = s.Metric[l]
	}
	return k
}

type mimirRow = map[string]any

// setCols writes the label columns. success and time_delay_ms are BOOLEAN and
// INTEGER in the table, so an empty value is left NULL instead of "".
func setCols(r mimirRow, cols []mimirCol, key []string) {
	for i, c := range cols {
		if key[i] == "" && (c.name == "success" || c.name == "time_delay_ms") {
			continue
		}
		r[c.name] = key[i]
	}
}

func setVal(r mimirRow, field string, x float64) {
	if !math.IsNaN(x) && !math.IsInf(x, 0) {
		r[field] = x
	}
}

func mimirFamily(ctx context.Context, m *MimirClient, day string, t time.Time, s mimirSpec) ([]mimirRow, error) {
	labels := labelsOf(s.cols)
	by := strings.Join(labels, ",")
	type acc struct {
		key  []string
		vals map[string]float64
		devs map[string]bool
	}
	var order []string
	vals := map[string]*acc{}
	put := func(field, expr string) error {
		res, err := m.query(ctx, expr, t)
		if err != nil {
			return err
		}
		for _, r := range res {
			k := keyOf(r, labels)
			id := strings.Join(k, "\x00")
			a := vals[id]
			if a == nil {
				a = &acc{key: k, vals: map[string]float64{}}
				vals[id] = a
				order = append(order, id)
			}
			a.vals[field] = r.float()
		}
		return nil
	}
	base := s.metric
	if s.hist {
		base += "_count"
		if err := put("hist_count", fmt.Sprintf("sum by (%s) (increase(%s_count[1d]))", by, s.metric)); err != nil {
			return nil, err
		}
		if err := put("hist_sum", fmt.Sprintf("sum by (%s) (increase(%s_sum[1d]))", by, s.metric)); err != nil {
			return nil, err
		}
	} else if err := put("increase", fmt.Sprintf("sum by (%s) (increase(%s[1d]))", by, s.metric)); err != nil {
		return nil, err
	}

	devs := map[string]map[string]bool{}
	res, err := m.query(ctx, fmt.Sprintf(`count by (%s,%s) (increase(%s{%s!=""}[1d]) > 0)`, by, mimirDevice, base, mimirDevice), t)
	if err != nil {
		return nil, err
	}
	for _, r := range res {
		id := strings.Join(keyOf(r, labels), "\x00")
		if devs[id] == nil {
			devs[id] = map[string]bool{}
		}
		devs[id][r.Metric[mimirDevice]] = true
	}

	mi := 0
	for i, c := range s.cols {
		if c.name == s.merge {
			mi = i
		}
	}
	var mergedOrder []string
	merged := map[string]*acc{}
	for _, id := range order {
		a := vals[id]
		d := devs[id]
		key := a.key
		if len(d) < mimirK {
			key = append([]string(nil), key...)
			key[mi] = "andre"
		}
		mid := strings.Join(key, "\x00")
		row := merged[mid]
		if row == nil {
			row = &acc{key: key, vals: map[string]float64{}, devs: map[string]bool{}}
			merged[mid] = row
			mergedOrder = append(mergedOrder, mid)
		}
		for dev := range d {
			row.devs[dev] = true
		}
		for f, x := range a.vals {
			row.vals[f] += x
		}
	}

	var out []mimirRow
	var keys [][]string
	for _, mid := range mergedOrder {
		row := merged[mid]
		if len(row.devs) < mimirK {
			continue
		}
		rec := mimirRow{"day": day, "family": s.family, "metric": s.metric, "devices": len(row.devs), "counter_reliable": s.reliable}
		setCols(rec, s.cols, row.key)
		for f, x := range row.vals {
			setVal(rec, f, x)
		}
		out = append(out, rec)
		keys = append(keys, row.key)
	}

	if s.hist && (s.family == "operation_duration" || s.family == "execute_tool") {
		// Quantiles only for unmerged rows; "andre" mixes populations.
		for _, p := range []struct {
			q     string
			field string
		}{{"0.5", "p50"}, {"0.95", "p95"}} {
			res, err := m.query(ctx, fmt.Sprintf("histogram_quantile(%s, sum by (%s,le) (increase(%s_bucket[1d])))", p.q, by, s.metric), t)
			if err != nil {
				return nil, err
			}
			qs := map[string]float64{}
			for _, r := range res {
				qs[strings.Join(keyOf(r, labels), "\x00")] = r.float()
			}
			for i, rec := range out {
				if keys[i][mi] == "andre" {
					continue
				}
				if x, ok := qs[strings.Join(keys[i], "\x00")]; ok {
					setVal(rec, p.field, x)
				}
			}
		}
	}
	return out, nil
}

func mimirRatios(day string, rows []mimirRow) []mimirRow {
	type jm struct{ job, model string }
	var order []jm
	agg := map[jm]map[string]float64{}
	devices := map[jm]any{}
	for _, r := range rows {
		if r["family"] != "inference_tokens" {
			continue
		}
		k := jm{r["job"].(string), r["model"].(string)}
		if agg[k] == nil {
			agg[k] = map[string]float64{}
			devices[k] = r["devices"]
			order = append(order, k)
		}
		x, _ := r["increase"].(float64)
		agg[k][r["metric"].(string)] = x
	}
	var out []mimirRow
	for _, k := range order {
		v := agg[k]
		in := v["gen_ai_client_inference_usage_input_tokens_total"]
		o := v["gen_ai_client_inference_usage_output_tokens_total"]
		c := v["gen_ai_client_inference_usage_cache_read_input_tokens_total"]
		if in > 0 && o > 0 {
			out = append(out, mimirRow{"day": day, "family": "inference_ratio", "metric": "ratios", "job": k.job, "model": k.model,
				"counter_reliable": false, "cache_read_share": c / in, "input_output_ratio": in / o, "devices": devices[k]})
		}
	}
	return out
}

func mimirChatRows(ctx context.Context, m *MimirClient, day string, t time.Time) ([]mimirRow, error) {
	var out []mimirRow
	for _, s := range mimirChat {
		labels := labelsOf(s.cols)
		fields := [][2]string{{"increase", s.metric}}
		if s.hist {
			fields = [][2]string{{"hist_count", s.metric + "_count"}, {"hist_sum", s.metric + "_sum"}}
		}
		var order []string
		rows := map[string]mimirRow{}
		for _, f := range fields {
			res, err := m.query(ctx, fmt.Sprintf("sum by (%s) (increase(%s[1d]))", strings.Join(labels, ","), f[1]), t)
			if err != nil {
				return nil, err
			}
			for _, r := range res {
				k := keyOf(r, labels)
				id := strings.Join(k, "\x00")
				if rows[id] == nil {
					rows[id] = mimirRow{"day": day, "family": "copilot_chat", "metric": s.metric, "counter_reliable": true}
					setCols(rows[id], s.cols, k)
					order = append(order, id)
				}
				setVal(rows[id], f[0], r.float())
			}
		}
		for _, id := range order {
			out = append(out, rows[id])
		}
	}
	return out, nil
}

func mimirConfigRows(ctx context.Context, m *MimirClient, day string, t time.Time) ([]mimirRow, error) {
	res, err := m.query(ctx, "count by (client) (count by (client, device_id) (last_over_time(nav_pilot_config_info[1d])))", t)
	if err != nil {
		return nil, err
	}
	var out []mimirRow
	for _, r := range res {
		if n := r.float(); n >= mimirK {
			out = append(out, mimirRow{"day": day, "family": "nav_pilot_config", "metric": "nav_pilot_config_info",
				"client": r.Metric["client"], "devices": int(n), "counter_reliable": true})
		}
	}
	return out, nil
}

// collectMimirDay returns all rows for one UTC day, evaluated at midnight after it.
func collectMimirDay(ctx context.Context, m *MimirClient, day time.Time) ([]mimirRow, error) {
	d := day.Format("2006-01-02")
	t := time.Date(day.Year(), day.Month(), day.Day()+1, 0, 0, 0, 0, time.UTC)
	var rows []mimirRow
	for _, s := range mimirSpecs {
		r, err := mimirFamily(ctx, m, d, t, s)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s.metric, err)
		}
		rows = append(rows, r...)
	}
	rows = append(rows, mimirRatios(d, rows)...)
	chat, err := mimirChatRows(ctx, m, d, t)
	if err != nil {
		return nil, fmt.Errorf("copilot_chat: %w", err)
	}
	cfg, err := mimirConfigRows(ctx, m, d, t)
	if err != nil {
		return nil, fmt.Errorf("nav_pilot_config: %w", err)
	}
	return append(append(rows, chat...), cfg...), nil
}

// runMimirDaily copies each day in [from, to] and replaces any earlier copy.
// A day with no rows is an error and leaves BigQuery untouched, so a rerun
// past Mimir's retention never wipes history.
func runMimirDaily(ctx context.Context, m *MimirClient, bq *BigQueryClient, from, to time.Time) error {
	for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
		rows, err := collectMimirDay(ctx, m, day)
		if err != nil {
			return fmt.Errorf("%s: %w", day.Format("2006-01-02"), err)
		}
		if len(rows) == 0 {
			return fmt.Errorf("%s: Mimir returned no rows", day.Format("2006-01-02"))
		}
		if err := bq.ReplaceMimirDay(ctx, day, rows); err != nil {
			return fmt.Errorf("%s: %w", day.Format("2006-01-02"), err)
		}
		slog.Info("Copied Mimir aggregates", "day", day.Format("2006-01-02"), "rows", len(rows))
	}
	return nil
}

var mimirDailySchema = bigquery.Schema{
	{Name: "day", Type: bigquery.DateFieldType},
	{Name: "family", Type: bigquery.StringFieldType},
	{Name: "metric", Type: bigquery.StringFieldType},
	{Name: "job", Type: bigquery.StringFieldType},
	{Name: "model", Type: bigquery.StringFieldType},
	{Name: "tool", Type: bigquery.StringFieldType},
	{Name: "token_type", Type: bigquery.StringFieldType},
	{Name: "operation", Type: bigquery.StringFieldType},
	{Name: "error_type", Type: bigquery.StringFieldType},
	{Name: "success", Type: bigquery.BooleanFieldType},
	{Name: "edit_outcome", Type: bigquery.StringFieldType},
	{Name: "edit_source", Type: bigquery.StringFieldType},
	{Name: "time_delay_ms", Type: bigquery.IntegerFieldType},
	{Name: "client", Type: bigquery.StringFieldType},
	{Name: "devices", Type: bigquery.IntegerFieldType},
	{Name: "counter_reliable", Type: bigquery.BooleanFieldType},
	{Name: "increase", Type: bigquery.FloatFieldType},
	{Name: "hist_count", Type: bigquery.FloatFieldType},
	{Name: "hist_sum", Type: bigquery.FloatFieldType},
	{Name: "p50", Type: bigquery.FloatFieldType},
	{Name: "p95", Type: bigquery.FloatFieldType},
	{Name: "cache_read_share", Type: bigquery.FloatFieldType},
	{Name: "input_output_ratio", Type: bigquery.FloatFieldType},
}

func (c *BigQueryClient) EnsureMimirDailyTableExists(ctx context.Context) error {
	t := c.client.Dataset(c.dataset).Table(mimirDailyName)
	if _, err := t.Metadata(ctx); err == nil {
		return nil
	}
	return t.Create(ctx, &bigquery.TableMetadata{Schema: mimirDailySchema, Description: "Daily Mimir aggregates (nav-pilot / Copilot telemetry). Device-labelled families have k>=5 devices; copilot_chat rows carry no device count."})
}

// ReplaceMimirDay loads rows into a scratch table, then swaps the day in one
// transaction, so a failed load never leaves the day empty.
func (c *BigQueryClient) ReplaceMimirDay(ctx context.Context, day time.Time, rows []mimirRow) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, r := range rows {
		if err := enc.Encode(r); err != nil {
			return err
		}
	}
	scratch := c.client.Dataset(c.dataset).Table(mimirDailyName + "_load_" + day.Format("20060102"))
	if err := scratch.Create(ctx, &bigquery.TableMetadata{Schema: mimirDailySchema, ExpirationTime: time.Now().Add(6 * time.Hour)}); err != nil && !strings.Contains(err.Error(), "Already Exists") {
		return err
	}
	src := bigquery.NewReaderSource(&buf)
	src.SourceFormat = bigquery.JSON
	src.Schema = mimirDailySchema
	l := scratch.LoaderFrom(src)
	l.WriteDisposition = bigquery.WriteTruncate
	if err := waitJob(ctx, l.Run); err != nil {
		return fmt.Errorf("load scratch table: %w", err)
	}
	cols := make([]string, len(mimirDailySchema))
	for i, f := range mimirDailySchema {
		cols[i] = f.Name
	}
	colList := strings.Join(cols, ", ")
	target := "`" + c.projectID + "." + c.dataset + "." + mimirDailyName + "`"
	q := c.client.Query("BEGIN TRANSACTION;\n" +
		"DELETE FROM " + target + " WHERE day = @day;\n" +
		"INSERT INTO " + target + " (" + colList + ") SELECT " + colList + " FROM `" + c.projectID + "." + c.dataset + "." + scratch.TableID + "`;\n" +
		"COMMIT TRANSACTION;")
	q.Parameters = []bigquery.QueryParameter{{Name: "day", Value: civil.DateOf(day)}}
	if err := waitJob(ctx, q.Run); err != nil {
		return fmt.Errorf("swap day: %w", err)
	}
	if err := scratch.Delete(ctx); err != nil {
		slog.Warn("Could not drop scratch table; it expires in 6 h", "table", scratch.TableID, "error", err)
	}
	return nil
}

func waitJob(ctx context.Context, run func(context.Context) (*bigquery.Job, error)) error {
	job, err := run(ctx)
	if err != nil {
		return err
	}
	status, err := job.Wait(ctx)
	if err != nil {
		return err
	}
	return status.Err()
}
