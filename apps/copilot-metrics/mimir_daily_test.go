package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

var codeLinesAdded = mimirSpec{"code_lines", "github_copilot_code_lines_added_total", jobModel(), "model", false, false}

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
