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
	device         bool
}

func jobModel() []mimirCol { return []mimirCol{{"job", "job"}, {"model", mimirModel}} }

// Copilot sessions running concurrently share one series, so Prometheus reads
// every drop as a counter reset and increase() overcounts. Token counters are
// known to be hit; tool-call and code-lines counters come from the same
// exporter and are flagged too.
var mimirSpecs = func() []mimirSpec {
	s := []mimirSpec{{family: "token_usage", metric: "gen_ai_client_token_usage", cols: append(jobModel(), mimirCol{"token_type", "gen_ai_token_type"}), merge: "model", hist: true}}
	for _, t := range []string{"input", "output", "cache_read_input", "cache_write_input", "reasoning_output"} {
		s = append(s, mimirSpec{family: "inference_tokens", metric: "gen_ai_client_inference_usage_" + t + "_tokens_total", cols: jobModel(), merge: "model"})
	}
	tool := []mimirCol{{"job", "job"}, {"tool", "gen_ai_tool_name"}}
	return append(s,
		mimirSpec{family: "operation_duration", metric: "gen_ai_client_operation_duration_seconds", cols: append(jobModel(), mimirCol{"operation", "gen_ai_operation_name"}, mimirCol{"error_type", "error_type"}), merge: "model", hist: true, reliable: true},
		mimirSpec{family: "execute_tool", metric: "gen_ai_execute_tool_duration_seconds", cols: append(tool, mimirCol{"error_type", "error_type"}), merge: "tool", hist: true, reliable: true},
		mimirSpec{family: "tool_call", metric: "github_copilot_tool_call_duration_seconds", cols: tool, merge: "tool", hist: true, reliable: true},
		mimirSpec{family: "tool_call", metric: "github_copilot_tool_call_count_total", cols: append(tool, mimirCol{"success", "success"}), merge: "tool"},
		mimirSpec{family: "code_lines", metric: "github_copilot_code_lines_added_total", cols: jobModel(), merge: "model"},
		mimirSpec{family: "code_lines", metric: "github_copilot_code_lines_removed_total", cols: jobModel(), merge: "model"},
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

func sessionCols(extra ...mimirCol) []mimirCol {
	return append([]mimirCol{{"version", "version"}, {"client", "client"}}, extra...)
}

func clientCols(extra ...mimirCol) []mimirCol {
	return append([]mimirCol{{"client", "client"}}, extra...)
}

// minSessionVersionRe matches release versions >= 2026.09.29-063952 (excludes "dev" and older releases).
// Uses [.] rather than \. because it is interpolated into a double-quoted PromQL string literal.
const minSessionVersionRe = `2026[.]09[.](29-(06395[2-9]|0639[6-9]|06[4-9]|0[7-9]|[12])|30).*|2026[.]1[0-2][.].*|20(2[7-9]|[3-9][0-9])[.].*`

// Session series come from newOnlyExporter (one delta per finished session),
// so sum_over_time adds sessions up without counter-reset artifacts. Instruments
// from copilot_session.go carry no device_id and group by version; instruments
// carrying device_id (device: true) filter version >= 2026.09.29-063952 in PromQL
// and omit version from the group-by so multiple daily releases do not split a
// day's cohort below k>=5.
var mimirSession = []mimirSpec{
	{metric: "nav_pilot_session_skill_loads_total", cols: sessionCols(mimirCol{"skill", "skill"}, mimirCol{"source", "source"}, mimirCol{"trigger", "trigger"})},
	{metric: "nav_pilot_session_tool_calls_total", cols: sessionCols(mimirCol{"tool", "tool"}, mimirCol{"outcome", "outcome"})},
	{metric: "nav_pilot_session_hook_runs_total", cols: sessionCols(mimirCol{"hook_type", "hook_type"}, mimirCol{"outcome", "outcome"})},
	{metric: "nav_pilot_subagent_selected_total", cols: sessionCols(mimirCol{"agent", "agent"})},
	{metric: "nav_pilot_session_tokens_total", cols: sessionCols(mimirCol{"model", "model"}, mimirCol{"token_type", "kind"})},
	{metric: "nav_pilot_session_premium_requests_total", cols: sessionCols(mimirCol{"model", "model"})},
	{metric: "nav_pilot_session_cost_nano_aiu_total", cols: sessionCols(mimirCol{"model", "model"})},
	{metric: "nav_pilot_session_turns", cols: sessionCols(), hist: true},
	{metric: "nav_pilot_session_duration_seconds", cols: sessionCols(mimirCol{"shutdown_type", "shutdown_type"}), hist: true},
	{metric: "nav_pilot_session_api_duration_ms", cols: sessionCols(mimirCol{"model", "model"}), hist: true},
	{metric: "nav_pilot_session_events_skipped_total", cols: sessionCols()},
	{metric: "nav_pilot_genai_session_tokens_total", cols: clientCols(mimirCol{"source", "gen_ai_provider_name"}, mimirCol{"model", "gen_ai_request_model"}, mimirCol{"token_type", "gen_ai_token_type"}), device: true},
	{metric: "nav_pilot_genai_session_calls_total", cols: clientCols(mimirCol{"source", "gen_ai_provider_name"}, mimirCol{"model", "gen_ai_request_model"}), device: true},
	{metric: "nav_pilot_local_dispatches", cols: clientCols(mimirCol{"model", "model"}, mimirCol{"operation", "dispatch_level"}, mimirCol{"outcome", "saw_traffic"}), hist: true, device: true},
	{metric: "nav_pilot_local_gate_total", cols: []mimirCol{{"outcome", "outcome"}}, device: true},
	{metric: "nav_pilot_local_ready_seconds", cols: []mimirCol{{"outcome", "outcome"}}, hist: true, device: true},
	{metric: "nav_pilot_decide_result_total", cols: []mimirCol{{"model", "model"}, {"outcome", "result"}, {"source", "caller"}, {"operation", "backend"}}, merge: "model", device: true},
	{metric: "nav_pilot_launch_error_total", cols: clientCols(mimirCol{"error_type", "error_type"}), device: true},
	{metric: "nav_pilot_sync_updates_total", cols: []mimirCol{{"source", "scope"}, {"operation", "mode"}}, device: true},
	{metric: "nav_pilot_sync_conflicts_total", cols: []mimirCol{{"source", "scope"}, {"operation", "mode"}}, device: true},
	{metric: "nav_pilot_hook_redact_total", cols: []mimirCol{{"outcome", "kind"}}, device: true},
	{metric: "nav_pilot_hook_loop_guard_total", cols: []mimirCol{{"operation", "rule"}, {"source", "session"}}, device: true},
	{metric: "nav_pilot_hook_action_check_total", cols: []mimirCol{{"outcome", "outcome"}, {"operation", "category"}}, device: true},
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
					rows[id] = mimirRow{"day": day, "family": "copilot_chat", "metric": s.metric, "counter_reliable": false}
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

func mimirSessionRows(ctx context.Context, m *MimirClient, day string, t time.Time) ([]mimirRow, error) {
	var out []mimirRow
	for _, s := range mimirSession {
		labels := labelsOf(s.cols)
		by := strings.Join(labels, ",")
		sel := ""
		if s.device {
			sel = `{version=~"` + minSessionVersionRe + `"}`
		}
		queries := [][2]string{{"increase", fmt.Sprintf("sum by (%s) (sum_over_time(%s%s[1d]))", by, s.metric, sel)}}
		if s.hist {
			queries = [][2]string{
				{"hist_count", fmt.Sprintf("sum by (%s) (sum_over_time(%s_count%s[1d]))", by, s.metric, sel)},
				{"hist_sum", fmt.Sprintf("sum by (%s) (sum_over_time(%s_sum%s[1d]))", by, s.metric, sel)},
				{"p50", fmt.Sprintf("histogram_quantile(0.5, sum by (%s,le) (sum_over_time(%s_bucket%s[1d])))", by, s.metric, sel)},
				{"p95", fmt.Sprintf("histogram_quantile(0.95, sum by (%s,le) (sum_over_time(%s_bucket%s[1d])))", by, s.metric, sel)},
			}
		}
		var order []string
		rows := map[string]mimirRow{}
		for _, q := range queries {
			res, err := m.query(ctx, q[1], t)
			if err != nil {
				return nil, err
			}
			for _, r := range res {
				v := r.float()
				if math.IsNaN(v) || math.IsInf(v, 0) {
					continue
				}
				k := keyOf(r, labels)
				id := strings.Join(k, "\x00")
				if rows[id] == nil {
					rows[id] = mimirRow{"day": day, "family": "nav_pilot_session", "metric": s.metric, "counter_reliable": true}
					setCols(rows[id], s.cols, k)
					order = append(order, id)
				}
				rows[id][q[0]] = v
			}
		}
		if s.device && len(rows) > 0 {
			base := s.metric
			if s.hist {
				base += "_count"
			}
			if s.merge != "" {
				expr := fmt.Sprintf(`count by (%s) (count by (%s,device_id) (sum_over_time(%s{device_id!="",version=~"%s"}[7d]) > 0))`, by, by, base, minSessionVersionRe)
				res, err := m.query(ctx, expr, t)
				if err != nil {
					return nil, err
				}
				devs := map[string]int{}
				for _, r := range res {
					if n := r.float(); n >= mimirK && !math.IsNaN(n) {
						devs[strings.Join(keyOf(r, labels), "\x00")] = int(n)
					}
				}
				for _, id := range order {
					if n, ok := devs[id]; ok {
						rows[id]["devices"] = n
						out = append(out, rows[id])
					}
				}
				continue
			}
			expr := fmt.Sprintf(`count(count by (device_id) (sum_over_time(%s{device_id!="",version=~"%s"}[7d]) > 0))`, base, minSessionVersionRe)
			res, err := m.query(ctx, expr, t)
			if err != nil {
				return nil, err
			}
			if len(res) > 0 {
				if n := res[0].float(); n >= mimirK && !math.IsNaN(n) {
					for _, id := range order {
						rows[id]["devices"] = int(n)
						out = append(out, rows[id])
					}
				}
			}
			continue
		}
		for _, id := range order {
			out = append(out, rows[id])
		}
	}
	return out, nil
}

func mimirConfigRows(ctx context.Context, m *MimirClient, day string, t time.Time) ([]mimirRow, error) {
	res, err := m.query(ctx, `count by (client) (count by (client, device_id) (last_over_time(nav_pilot_config_info{execution_context="organic",version!="dev"}[1d])))`, t)
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

func mimirReachRows(ctx context.Context, m *MimirClient, day string, t time.Time) ([]mimirRow, error) {
	const sel = `execution_context="organic",version!="dev"`
	specs := []struct {
		metric, expr, op, fixedClient, outcome string
		cols                                   []mimirCol
	}{
		{metric: "nav_pilot_info", op: "1d", expr: `count(count by (device_id) (last_over_time(nav_pilot_info{` + sel + `}[1d])))`},
		{metric: "nav_pilot_info", op: "7d", expr: `count(count by (device_id) (last_over_time(nav_pilot_info{` + sel + `}[7d])))`},
		{metric: "nav_pilot_info", op: "28d", expr: `count(count by (device_id) (last_over_time(nav_pilot_info{` + sel + `}[28d])))`},
		{metric: "nav_pilot_info", op: "rtk_7d", cols: []mimirCol{{"outcome", "rtk_installed"}}, expr: `count by (rtk_installed) (count by (rtk_installed, device_id) (last_over_time(nav_pilot_info{` + sel + `,rtk_installed!=""}[7d])))`},
		{metric: "nav_pilot_config_info", op: "7d", cols: []mimirCol{{"client", "client"}}, expr: `count by (client) (count by (client, device_id) (last_over_time(nav_pilot_config_info{` + sel + `}[7d])))`},
		{metric: "nav_pilot_config_info", op: "7d", fixedClient: "both", expr: `count(count by (device_id) (count by (client, device_id) (last_over_time(nav_pilot_config_info{` + sel + `,client=~"copilot|opencode"}[7d]))) > 1)`},
		{metric: "nav_pilot_client_available", op: "7d", cols: []mimirCol{{"client", "client"}}, expr: `count by (client) (max by (client, device_id) (last_over_time(nav_pilot_client_available{` + sel + `}[7d])) == 1)`},
		{metric: "nav_pilot_command_duration_ms_count", cols: []mimirCol{{"operation", "command"}}, expr: `count by (command) (count by (command, device_id) (last_over_time(nav_pilot_command_duration_ms_count{` + sel + `}[7d])))`},
		{metric: "nav_pilot_installed_items", op: "7d", cols: []mimirCol{{"source", "scope"}}, expr: `count by (scope) (count by (scope, device_id) (last_over_time(nav_pilot_installed_items{` + sel + `,status="active"}[7d])))`},
		{metric: "nav_pilot_install_present", cols: []mimirCol{{"source", "scope"}, {"operation", "collection"}}, expr: `count by (scope, collection) (max by (scope, collection, device_id) (last_over_time(nav_pilot_install_present{` + sel + `}[7d])) == 1)`},
		{metric: "nav_pilot_local_dispatches_count", op: "7d", expr: `count(count by (device_id) (last_over_time(nav_pilot_local_dispatches_count{device_id!="",version!="dev"}[7d])))`},
	}
	var out []mimirRow
	for _, s := range specs {
		res, err := m.query(ctx, s.expr, t)
		if err != nil {
			return nil, err
		}
		labels := labelsOf(s.cols)
		for _, r := range res {
			n := r.float()
			if n < mimirK || math.IsNaN(n) {
				continue
			}
			row := mimirRow{"day": day, "family": "nav_pilot_reach", "metric": s.metric, "devices": int(n), "counter_reliable": true}
			if s.op != "" {
				row["operation"] = s.op
			}
			if s.fixedClient != "" {
				row["client"] = s.fixedClient
			}
			if s.outcome != "" {
				row["outcome"] = s.outcome
			}
			setCols(row, s.cols, keyOf(r, labels))
			out = append(out, row)
		}
	}
	res, err := m.query(ctx, `max by (component, scope, version, device_id) (last_over_time(nav_pilot_up_to_date{`+sel+`,device_id!=""}[7d]))`, t)
	if err != nil {
		return nil, err
	}
	type devVer struct {
		ver string
		val float64
	}
	type csKey struct{ comp, scope string }
	latest := map[csKey]map[string]devVer{}
	var csOrder []csKey
	for _, r := range res {
		v := r.float()
		if math.IsNaN(v) {
			continue
		}
		k := csKey{r.Metric["component"], r.Metric["scope"]}
		if latest[k] == nil {
			latest[k] = map[string]devVer{}
			csOrder = append(csOrder, k)
		}
		dev := r.Metric["device_id"]
		ver := r.Metric["version"]
		if cur, ok := latest[k][dev]; !ok || ver > cur.ver || (ver == cur.ver && v > cur.val) {
			latest[k][dev] = devVer{ver: ver, val: v}
		}
	}
	for _, k := range csOrder {
		var up, stale int
		for _, dv := range latest[k] {
			switch dv.val {
			case 1:
				up++
			case 0:
				stale++
			}
		}
		for _, o := range []struct {
			outcome string
			n       int
		}{{"up_to_date", up}, {"stale", stale}} {
			if o.n >= mimirK {
				out = append(out, mimirRow{"day": day, "family": "nav_pilot_reach", "metric": "nav_pilot_up_to_date",
					"operation": k.comp, "source": k.scope, "outcome": o.outcome, "devices": o.n, "counter_reliable": true})
			}
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
	reach, err := mimirReachRows(ctx, m, d, t)
	if err != nil {
		return nil, fmt.Errorf("nav_pilot_reach: %w", err)
	}
	sess, err := mimirSessionRows(ctx, m, d, t)
	if err != nil {
		return nil, fmt.Errorf("nav_pilot_session: %w", err)
	}
	return append(append(append(append(rows, chat...), cfg...), reach...), sess...), nil
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
	{Name: "version", Type: bigquery.StringFieldType},
	{Name: "skill", Type: bigquery.StringFieldType},
	{Name: "source", Type: bigquery.StringFieldType},
	{Name: "trigger", Type: bigquery.StringFieldType},
	{Name: "outcome", Type: bigquery.StringFieldType},
	{Name: "hook_type", Type: bigquery.StringFieldType},
	{Name: "agent", Type: bigquery.StringFieldType},
	{Name: "shutdown_type", Type: bigquery.StringFieldType},
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
	if meta, err := t.Metadata(ctx); err == nil {
		if len(meta.Schema) < len(mimirDailySchema) {
			_, err = t.Update(ctx, bigquery.TableMetadataToUpdate{Schema: mimirDailySchema}, meta.ETag)
		}
		return err
	}
	return t.Create(ctx, &bigquery.TableMetadata{Schema: mimirDailySchema, Description: "Daily Mimir aggregates (nav-pilot / Copilot telemetry). Device-labelled families have k>=5 devices; copilot_chat and nav_pilot_session rows carry no device count."})
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
