package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

type fakeSeries struct {
	labels map[string]string
	value  string
}

// fakeMimir answers each query with the series registered for the first
// matching substring, and records every query and its time.
func fakeMimir(t *testing.T, answers map[string][]fakeSeries) (*MimirClient, *[]string) {
	t.Helper()
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Scope-OrgID") != "nais" {
			http.Error(w, "no org", http.StatusUnauthorized)
			return
		}
		q := r.URL.Query().Get("query")
		seen = append(seen, q+" @"+r.URL.Query().Get("time"))
		var res []map[string]any
		for sub, series := range answers {
			if strings.Contains(q, sub) {
				for _, s := range series {
					res = append(res, map[string]any{"metric": s.labels, "value": []any{0, s.value}})
				}
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"resultType": "vector", "result": res}})
	}))
	t.Cleanup(srv.Close)
	return &MimirClient{BaseURL: srv.URL, HTTP: srv.Client()}, &seen
}

func devices(job, model string, n int) []fakeSeries {
	var out []fakeSeries
	for i := 0; i < n; i++ {
		out = append(out, fakeSeries{map[string]string{"job": job, mimirModel: model, mimirDevice: fmt.Sprintf("%s-%d", model, i)}, "1"})
	}
	return out
}

var codeLinesAdded = mimirSpec{family: "code_lines", metric: "github_copilot_code_lines_added_total", cols: jobModel(), merge: "model"}

func TestMimirFamilyQueriesAndMerge(t *testing.T) {
	val := func(model, v string) fakeSeries {
		return fakeSeries{map[string]string{"job": "nav-pilot", mimirModel: model}, v}
	}
	var devs []fakeSeries
	devs = append(devs, devices("nav-pilot", "big", 6)...)
	devs = append(devs, devices("nav-pilot", "b", 2)...)
	devs = append(devs, devices("nav-pilot", "c", 3)...)
	m, seen := fakeMimir(t, map[string][]fakeSeries{
		"sum by (job,gen_ai_request_model) (increase(github_copilot_code_lines_added_total[1d]))": {val("big", "100"), val("b", "10"), val("c", "5")},
		"count by (job,gen_ai_request_model,nav_pilot_device_id)":                                 devs,
	})
	tm := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	rows, err := mimirFamily(context.Background(), m, "2026-10-09", tm, codeLinesAdded)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"sum by (job,gen_ai_request_model) (increase(github_copilot_code_lines_added_total[1d])) @2026-10-10T00:00:00Z",
		`count by (job,gen_ai_request_model,nav_pilot_device_id) (increase(github_copilot_code_lines_added_total{nav_pilot_device_id!=""}[1d]) > 0) @2026-10-10T00:00:00Z`,
	}
	if strings.Join(*seen, "\n") != strings.Join(want, "\n") {
		t.Fatalf("queries:\n%s", strings.Join(*seen, "\n"))
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %v", rows)
	}
	if rows[0]["model"] != "big" || rows[0]["devices"] != 6 || rows[0]["increase"] != 100.0 {
		t.Errorf("big row = %v", rows[0])
	}
	// b (2 devices) and c (3) merge into one "andre" row with 5 devices.
	if rows[1]["model"] != "andre" || rows[1]["devices"] != 5 || rows[1]["increase"] != 15.0 {
		t.Errorf("andre row = %v", rows[1])
	}
	if rows[0]["counter_reliable"] != false {
		t.Error("code_lines must be flagged unreliable")
	}
	for _, r := range rows {
		if _, ok := r[mimirDevice]; ok {
			t.Error("device id leaked into row")
		}
	}
}

func TestMimirFamilyDropsSmallAndre(t *testing.T) {
	m, _ := fakeMimir(t, map[string][]fakeSeries{
		"sum by":   {{map[string]string{"job": "j", mimirModel: "x"}, "3"}},
		"count by": devices("j", "x", 4),
	})
	rows, err := mimirFamily(context.Background(), m, "2026-10-09", time.Now(), codeLinesAdded)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("an andre row under 5 devices must be dropped, got %v", rows)
	}
}

func TestMimirHistQuantilesSkipAndre(t *testing.T) {
	spec := mimirSpecs[6] // operation_duration
	if spec.family != "operation_duration" {
		t.Fatalf("spec order changed: %s", spec.family)
	}
	lab := func(model string) map[string]string {
		return map[string]string{"job": "j", mimirModel: model, "gen_ai_operation_name": "chat", "error_type": ""}
	}
	var devs []fakeSeries
	for _, d := range append(devices("j", "big", 5), devices("j", "small", 5)[:1]...) {
		l := lab(d.labels[mimirModel])
		l[mimirDevice] = d.labels[mimirDevice]
		devs = append(devs, fakeSeries{l, "1"})
	}
	m, seen := fakeMimir(t, map[string][]fakeSeries{
		"_count[1d]))":             {{lab("big"), "10"}, {lab("small"), "1"}},
		"_sum[1d]))":               {{lab("big"), "20"}},
		"count by":                 devs,
		"histogram_quantile(0.5,":  {{lab("big"), "1.5"}, {lab("andre"), "9"}},
		"histogram_quantile(0.95,": {{lab("big"), "NaN"}},
	})
	rows, err := mimirFamily(context.Background(), m, "d", time.Now(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["p50"] != 1.5 || rows[0]["hist_sum"] != 20.0 || rows[0]["operation"] != "chat" {
		t.Fatalf("rows = %v", rows)
	}
	if _, ok := rows[0]["p95"]; ok {
		t.Error("NaN quantile must be left out")
	}
	if !strings.Contains(strings.Join(*seen, "\n"), "histogram_quantile(0.95, sum by (job,gen_ai_request_model,gen_ai_operation_name,error_type,le) (increase(gen_ai_client_operation_duration_seconds_bucket[1d])))") {
		t.Errorf("quantile query missing:\n%s", strings.Join(*seen, "\n"))
	}
}

func TestMimirReliabilityFlags(t *testing.T) {
	for _, s := range mimirSpecs {
		unreliable := s.family == "token_usage" || s.family == "inference_tokens" || s.family == "code_lines" || s.metric == "github_copilot_tool_call_count_total"
		if s.reliable == unreliable {
			t.Errorf("%s reliable=%v", s.metric, s.reliable)
		}
	}
}

func TestMimirRatios(t *testing.T) {
	rows := []mimirRow{
		{"family": "inference_tokens", "metric": "gen_ai_client_inference_usage_input_tokens_total", "job": "j", "model": "m", "devices": 7, "increase": 100.0},
		{"family": "inference_tokens", "metric": "gen_ai_client_inference_usage_output_tokens_total", "job": "j", "model": "m", "devices": 6, "increase": 25.0},
		{"family": "inference_tokens", "metric": "gen_ai_client_inference_usage_cache_read_input_tokens_total", "job": "j", "model": "m", "devices": 5, "increase": 40.0},
	}
	r := mimirRatios("d", rows)
	if len(r) != 1 || r[0]["cache_read_share"] != 0.4 || r[0]["input_output_ratio"] != 4.0 || r[0]["devices"] != 7 || r[0]["counter_reliable"] != false {
		t.Fatalf("ratios = %v", r)
	}
}

func TestMimirSessionRows(t *testing.T) {
	schemaCols := map[string]bool{}
	for _, f := range mimirDailySchema {
		schemaCols[f.Name] = true
	}
	for _, s := range mimirSession {
		for _, c := range s.cols {
			if !schemaCols[c.name] {
				t.Fatalf("%s column %q missing from mimirDailySchema", s.metric, c.name)
			}
		}
	}

	re := regexp.MustCompile("^(?:" + minSessionVersionRe + ")$")
	for ver, want := range map[string]bool{
		"":                      false,
		"dev":                   false,
		"0.18.0":                false,
		"2026.08.31-120000-abc": false,
		"2026.09.28-235959-abc": false,
		"2026.09.29-063951-abc": false,
		"2026X09X29-063952-abc": false,
		"2026.09.29-063952":     true,
		"2026.09.29-063952-abc": true,
		"2026.09.29-064000-abc": true,
		"2026.09.30-000000-abc": true,
		"2026.10.14-120000-abc": true,
		"2027.01.01-000000-abc": true,
	} {
		if got := re.MatchString(ver); got != want {
			t.Fatalf("minSessionVersionRe.Match(%q) = %v, want %v", ver, got, want)
		}
	}
	if strings.Contains(minSessionVersionRe, `\`) {
		t.Fatalf("minSessionVersionRe must not contain raw backslashes (PromQL string literal escape error): %s", minSessionVersionRe)
	}

	devSel := `{version=~"` + minSessionVersionRe + `"}`
	devCountSel := `{device_id!="",version=~"` + minSessionVersionRe + `"}`
	m, seen := fakeMimir(t, map[string][]fakeSeries{
		"nav_pilot_session_skill_loads_total[1d]": {
			{map[string]string{"version": "0.18.0", "client": "copilot", "skill": "aksel", "source": "nav", "trigger": "auto"}, "12"},
		},
		"nav_pilot_session_tokens_total[1d]": {
			{map[string]string{"version": "0.18.0", "client": "copilot", "model": "claude-sonnet-4.5", "kind": "input"}, "5000"},
		},
		"nav_pilot_subagent_selected_total[1d]": {
			{map[string]string{"version": "0.18.0", "client": "copilot", "agent": "security"}, "4"},
		},
		"nav_pilot_session_turns_count[1d]": {
			{map[string]string{"version": "0.18.0", "client": "copilot"}, "20"},
		},
		"nav_pilot_session_turns_sum[1d]": {
			{map[string]string{"version": "0.18.0", "client": "copilot"}, "140"},
		},
		"histogram_quantile(0.5, sum by (version,client,le) (sum_over_time(nav_pilot_session_turns_bucket[1d])))": {
			{map[string]string{"version": "0.18.0", "client": "copilot"}, "6"},
		},
		"histogram_quantile(0.95, sum by (version,client,le) (sum_over_time(nav_pilot_session_turns_bucket[1d])))": {
			{map[string]string{"version": "0.18.0", "client": "copilot"}, "NaN"},
		},
		"nav_pilot_local_dispatches_count" + devSel + "[1d]": {
			{map[string]string{"client": "opencode", "model": "qwen3-coder", "dispatch_level": "subagent", "saw_traffic": "true"}, "2"},
			{map[string]string{"client": "opencode", "model": "qwen3-coder", "dispatch_level": "subagent", "saw_traffic": "false"}, "14"},
		},
		"nav_pilot_local_dispatches_sum" + devSel + "[1d]": {
			{map[string]string{"client": "opencode", "model": "qwen3-coder", "dispatch_level": "subagent", "saw_traffic": "true"}, "57"},
			{map[string]string{"client": "opencode", "model": "qwen3-coder", "dispatch_level": "subagent", "saw_traffic": "false"}, "0"},
		},
		"nav_pilot_local_dispatches_count" + devCountSel + "[7d]": {
			{map[string]string{}, "6"},
		},
		"nav_pilot_decide_result_total" + devSel + "[1d]": {
			{map[string]string{"model": "qwen3-coder", "result": "local", "caller": "opencode", "backend": "ollama"}, "18"},
			{map[string]string{"model": "rare-model", "result": "local", "caller": "opencode", "backend": "ollama"}, "2"},
		},
		"nav_pilot_decide_result_total" + devCountSel + "[7d]": {
			{map[string]string{"model": "qwen3-coder", "result": "local", "caller": "opencode", "backend": "ollama"}, "7"},
			{map[string]string{"model": "rare-model", "result": "local", "caller": "opencode", "backend": "ollama"}, "3"},
		},
		"nav_pilot_launch_error_total" + devSel + "[1d]": {
			{map[string]string{"client": "opencode", "error_type": "exit_code"}, "14"},
		},
		"nav_pilot_launch_error_total" + devCountSel + "[7d]": {
			{map[string]string{}, "10"},
		},
	})
	rows, err := mimirSessionRows(context.Background(), m, "2026-10-09", time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range *seen {
		if !strings.Contains(q, "sum_over_time(") || strings.Contains(q, "increase(") {
			t.Fatalf("session query must use sum_over_time, got %s", q)
		}
		if !strings.Contains(q, "version") {
			t.Fatalf("session query must reference version, got %s", q)
		}
	}
	if len(rows) != 8 {
		t.Fatalf("expected 8 rows (both saw_traffic outcomes kept, rare-model <5 devices dropped), got %d: %v", len(rows), rows)
	}
	byMetric := map[string]mimirRow{}
	dispByOutcome := map[string]mimirRow{}
	for _, r := range rows {
		byMetric[r["metric"].(string)] = r
		if r["metric"] == "nav_pilot_local_dispatches" {
			dispByOutcome[r["outcome"].(string)] = r
		}
		if r["family"] != "nav_pilot_session" || r["counter_reliable"] != true {
			t.Errorf("unexpected row metadata: %v", r)
		}
	}
	if byMetric["nav_pilot_session_skill_loads_total"]["skill"] != "aksel" || byMetric["nav_pilot_session_skill_loads_total"]["increase"] != 12.0 || byMetric["nav_pilot_session_skill_loads_total"]["version"] != "0.18.0" {
		t.Errorf("skill row = %v", byMetric["nav_pilot_session_skill_loads_total"])
	}
	if byMetric["nav_pilot_session_tokens_total"]["token_type"] != "input" || byMetric["nav_pilot_session_tokens_total"]["increase"] != 5000.0 {
		t.Errorf("tokens row = %v", byMetric["nav_pilot_session_tokens_total"])
	}
	if byMetric["nav_pilot_subagent_selected_total"]["agent"] != "security" || byMetric["nav_pilot_subagent_selected_total"]["increase"] != 4.0 {
		t.Errorf("subagent row = %v", byMetric["nav_pilot_subagent_selected_total"])
	}
	turns := byMetric["nav_pilot_session_turns"]
	if turns["hist_count"] != 20.0 || turns["hist_sum"] != 140.0 || turns["p50"] != 6.0 {
		t.Errorf("turns row = %v", turns)
	}
	if _, ok := turns["p95"]; ok {
		t.Errorf("NaN p95 must be omitted: %v", turns)
	}
	dispTrue := dispByOutcome["true"]
	dispFalse := dispByOutcome["false"]
	if dispTrue["hist_count"] != 2.0 || dispTrue["hist_sum"] != 57.0 || dispTrue["operation"] != "subagent" || dispTrue["devices"] != 6 {
		t.Errorf("local_dispatches true row = %v", dispTrue)
	}
	if dispFalse["hist_count"] != 14.0 || dispFalse["hist_sum"] != 0.0 || dispFalse["devices"] != 6 {
		t.Errorf("local_dispatches false row = %v", dispFalse)
	}
	if _, hasVer := dispTrue["version"]; hasVer {
		t.Errorf("device-gated session row must not split by version: %v", dispTrue)
	}
	decide := byMetric["nav_pilot_decide_result_total"]
	if decide["increase"] != 18.0 || decide["outcome"] != "local" || decide["source"] != "opencode" || decide["operation"] != "ollama" || decide["devices"] != 7 {
		t.Errorf("decide row = %v", decide)
	}
	launchErr := byMetric["nav_pilot_launch_error_total"]
	if launchErr["increase"] != 14.0 || launchErr["client"] != "opencode" || launchErr["error_type"] != "exit_code" || launchErr["devices"] != 10 {
		t.Errorf("launch_error row = %v", launchErr)
	}
}

func TestMimirReachRows(t *testing.T) {
	// 5 devices up-to-date on v2, plus 5 devices stale on v2 (one of which had 1 on older v1).
	var upToDateSeries []fakeSeries
	for i := 0; i < 5; i++ {
		upToDateSeries = append(upToDateSeries, fakeSeries{
			map[string]string{"component": "cli", "scope": "none", "version": "2026.10.14-120000", "device_id": fmt.Sprintf("up-%d", i)}, "1",
		})
	}
	upToDateSeries = append(upToDateSeries, fakeSeries{
		map[string]string{"component": "cli", "scope": "none", "version": "2026.10.10-100000", "device_id": "stale-0"}, "1",
	})
	for i := 0; i < 5; i++ {
		upToDateSeries = append(upToDateSeries, fakeSeries{
			map[string]string{"component": "cli", "scope": "none", "version": "2026.10.12-100000", "device_id": fmt.Sprintf("stale-%d", i)}, "0",
		})
	}
	m, seen := fakeMimir(t, map[string][]fakeSeries{
		`nav_pilot_info{execution_context="organic",version!="dev"}[7d]`: {
			{map[string]string{}, "217"},
		},
		`rtk_installed!=""}[7d]`: {
			{map[string]string{"rtk_installed": "true"}, "136"},
			{map[string]string{"rtk_installed": "false"}, "96"},
		},
		`count by (client) (count by (client, device_id) (last_over_time(nav_pilot_config_info{execution_context="organic",version!="dev"}[7d])))`: {
			{map[string]string{"client": "copilot"}, "160"},
			{map[string]string{"client": "opencode"}, "56"},
		},
		`client=~"copilot|opencode"}[7d]))) > 1)`: {
			{map[string]string{}, "13"},
		},
		`nav_pilot_client_available`: {
			{map[string]string{"client": "copilot"}, "195"},
			{map[string]string{"client": "opencode"}, "102"},
			{map[string]string{"client": "pi"}, "3"},
		},
		`nav_pilot_command_duration_ms_count`: {
			{map[string]string{"command": "sync"}, "180"},
			{map[string]string{"command": "rare"}, "3"},
		},
		`nav_pilot_installed_items`: {
			{map[string]string{"scope": "user"}, "162"},
			{map[string]string{"scope": "repo"}, "46"},
		},
		`nav_pilot_up_to_date{execution_context="organic",version!="dev",device_id!=""}[7d]`: upToDateSeries,
		`nav_pilot_local_dispatches_count{device_id!="",version!="dev"}[7d]`: {
			{map[string]string{}, "6"},
		},
	})
	rows, err := mimirReachRows(context.Background(), m, "2026-10-14", time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range *seen {
		if !strings.Contains(q, `version!="dev"`) {
			t.Fatalf("reach query missing non-dev filter: %s", q)
		}
	}
	if len(rows) != 14 {
		t.Fatalf("expected 14 reach rows (<5 dropped), got %d: %v", len(rows), rows)
	}
	for _, r := range rows {
		if _, ok := r["device_id"]; ok {
			t.Fatalf("device_id leaked into reach row: %v", r)
		}
		if r["metric"] == "nav_pilot_up_to_date" && r["devices"] != 5 {
			t.Errorf("expected 5 devices for %v, got %v", r["outcome"], r)
		}
	}
}
