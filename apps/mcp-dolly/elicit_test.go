package main

import (
	"bytes"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var formCaps = map[string]any{"elicitation": map[string]any{"form": map[string]any{}}}

// callElicit sends a 2026-07-28 request with the given client capabilities and
// optional MRTR retry fields. It returns the raw body and the decoded result.
func callElicit(t *testing.T, srv *server, method string, params, caps map[string]any) (string, map[string]any) {
	t.Helper()
	body := modernBody(method, maps.Clone(params))
	body["params"].(map[string]any)["_meta"].(map[string]any)[mcpMetaClientCapabilities] = caps
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer in")
	name, _ := params["name"].(string)
	for k, v := range modernHeaders(method, name) {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)
	var out struct {
		Result map[string]any `json:"result"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Body.String(), out.Result
}

func orderCall(extra map[string]any) map[string]any {
	p := map[string]any{"name": "create_order", "arguments": twoStepArgs}
	maps.Copy(p, extra)
	return p
}

func answer(action string, confirm bool) map[string]any {
	return map[string]any{orderConfirmKey: map[string]any{"action": action, "content": map[string]any{"confirm": confirm}}}
}

// modelVisible is what a client hands the model: tool definitions and CallToolResult content.
func modelVisible(res map[string]any) string {
	b, _ := json.Marshal(map[string]any{"content": res["content"], "structuredContent": res["structuredContent"], "tools": res["tools"]})
	return string(b)
}

func TestCreateOrderElicitationFlow(t *testing.T) {
	srv, posts, _ := twoStepServer(t)

	_, list := callElicit(t, srv, "tools/list", nil, formCaps)
	if s := modelVisible(list); strings.Contains(s, confirmTokenKey) || !strings.Contains(s, "create_order") {
		t.Fatalf("tools/list for an elicitation client must offer create_order without confirmationToken: %s", s)
	}

	_, first := callElicit(t, srv, "tools/call", orderCall(nil), formCaps)
	state, _ := first["requestState"].(string)
	if first["resultType"] != "input_required" || state == "" || posts() != 0 {
		t.Fatalf("first call must ask for input and order nothing: %+v posts=%d", first, posts())
	}
	if strings.Contains(modelVisible(first), state) {
		t.Fatal("token leaked outside requestState")
	}
	ir, _ := json.Marshal(first["inputRequests"])
	if strings.Contains(string(ir), state) || !strings.Contains(string(ir), "elicitation/create") {
		t.Fatalf("inputRequests must be an elicitation without the token: %s", ir)
	}

	raw, done := callElicit(t, srv, "tools/call", orderCall(map[string]any{"requestState": state, "inputResponses": answer("accept", true)}), formCaps)
	if done["resultType"] != "complete" || done["isError"] == true || posts() != 1 {
		t.Fatalf("accepted retry must order once: %s posts=%d", raw, posts())
	}
	if strings.Contains(raw, state) || strings.Contains(raw, confirmTokenKey) {
		t.Fatalf("final result must not carry the token: %s", raw)
	}

	// The same requestState is single use.
	_, again := callElicit(t, srv, "tools/call", orderCall(map[string]any{"requestState": state, "inputResponses": answer("accept", true)}), formCaps)
	if again["isError"] != true || posts() != 1 {
		t.Fatalf("replayed requestState must fail: %+v posts=%d", again, posts())
	}
}

func TestCreateOrderElicitationNotConfirmed(t *testing.T) {
	for name, resp := range map[string]map[string]any{
		"decline":         answer("decline", false),
		"cancel":          answer("cancel", false),
		"accept unticked": answer("accept", false),
		"decline ticked":  answer("decline", true),
		"cancel ticked":   answer("cancel", true),
		"accept missing":  {orderConfirmKey: map[string]any{"action": "accept", "content": map[string]any{}}},
	} {
		t.Run(name, func(t *testing.T) {
			srv, posts, _ := twoStepServer(t)
			_, first := callElicit(t, srv, "tools/call", orderCall(nil), formCaps)
			_, res := callElicit(t, srv, "tools/call", orderCall(map[string]any{"requestState": first["requestState"], "inputResponses": resp}), formCaps)
			if posts() != 0 || res["resultType"] != "complete" {
				t.Fatalf("unconfirmed order must not be sent: %+v posts=%d", res, posts())
			}
		})
	}
}

func TestCreateOrderElicitationRejectsModelToken(t *testing.T) {
	srv, posts, _ := twoStepServer(t)
	// A token obtained through the two-step flow (a client without elicitation).
	_, out := callToolModern(t, srv.routes(), "in", "create_order", twoStepArgs)
	tok := previewToken(t, out)
	if tok == "" {
		t.Fatal("no preview token")
	}
	args := maps.Clone(twoStepArgs)
	args[confirmTokenKey] = tok
	_, res := callElicit(t, srv, "tools/call", map[string]any{"name": "create_order", "arguments": args}, formCaps)
	if res["isError"] != true || posts() != 0 {
		t.Fatalf("elicitation client must not order on a model-supplied token: %+v posts=%d", res, posts())
	}
}

func TestCreateOrderNoElicitationKeepsTwoStep(t *testing.T) {
	for name, caps := range map[string]map[string]any{
		"none":     {},
		"url only": {"elicitation": map[string]any{"url": map[string]any{}}},
	} {
		t.Run(name, func(t *testing.T) {
			srv, _, _ := twoStepServer(t)
			_, res := callElicit(t, srv, "tools/call", orderCall(nil), caps)
			if res["resultType"] != "complete" || res["inputRequests"] != nil {
				t.Fatalf("server must not elicit from a client without form elicitation: %+v", res)
			}
		})
	}
}

func TestFormElicitationCapability(t *testing.T) {
	for raw, want := range map[string]bool{
		`{}`:                                   false,
		`{"elicitation":{}}`:                   true,
		`{"elicitation":{"form":{}}}`:          true,
		`{"elicitation":{"url":{}}}`:           false,
		`{"elicitation":null}`:                 false,
		`{"elicitation":{"form":{},"url":{}}}`: true,
	} {
		var e mcpEnvelope
		e.Params.Meta = map[string]json.RawMessage{mcpMetaClientCapabilities: json.RawMessage(raw)}
		if got := e.formElicitation(); got != want {
			t.Errorf("%s: got %v want %v", raw, got, want)
		}
	}
}

// Flag off (prod): a client without form elicitation gets no token and orders nothing,
// on either path; an elicitation client is unaffected.
func TestCreateOrderTwoStepFlagOff(t *testing.T) {
	srv, posts, _ := twoStepServer(t)
	// A token minted while the flag was on must not work after it is turned off.
	_, out := callToolModern(t, srv.routes(), "in", "create_order", twoStepArgs)
	tok := previewToken(t, out)
	srv.cfg.TwoStepOrder = false

	_, list := callElicit(t, srv, "tools/list", nil, map[string]any{})
	if s := modelVisible(list); strings.Contains(s, confirmTokenKey) {
		t.Fatalf("flag off: tools/list must not offer confirmationToken: %s", s)
	}
	withTok := maps.Clone(twoStepArgs)
	withTok[confirmTokenKey] = tok
	for name, args := range map[string]map[string]any{"step 1": twoStepArgs, "step 2": withTok} {
		_, res := callElicit(t, srv, "tools/call", map[string]any{"name": "create_order", "arguments": args}, map[string]any{})
		b, _ := json.Marshal(res)
		if res["isError"] != true || !strings.Contains(string(b), "1.0.95") || strings.Contains(string(b), confirmTokenKey) || posts() != 0 {
			t.Fatalf("flag off, %s: want refusal naming supported clients: %s posts=%d", name, b, posts())
		}
	}
	_, ro := callElicit(t, srv, "tools/call", map[string]any{"name": "dolly_current_user", "arguments": map[string]any{}}, map[string]any{})
	if ro["isError"] == true {
		t.Fatalf("flag off must not affect read-only tools: %+v", ro)
	}
	_, first := callElicit(t, srv, "tools/call", orderCall(nil), formCaps)
	if first["resultType"] != "input_required" {
		t.Fatalf("flag off must not affect elicitation clients: %+v", first)
	}
}

// A declined or cancelled state is spent: replaying it as accepted orders nothing.
func TestCreateOrderElicitationNoReplayAfterDecline(t *testing.T) {
	for _, action := range []string{"decline", "cancel"} {
		t.Run(action, func(t *testing.T) {
			srv, posts, _ := twoStepServer(t)
			_, first := callElicit(t, srv, "tools/call", orderCall(nil), formCaps)
			st := first["requestState"]
			callElicit(t, srv, "tools/call", orderCall(map[string]any{"requestState": st, "inputResponses": answer(action, false)}), formCaps)
			_, res := callElicit(t, srv, "tools/call", orderCall(map[string]any{"requestState": st, "inputResponses": answer("accept", true)}), formCaps)
			if res["isError"] != true || posts() != 0 {
				t.Fatalf("replay after %s must be refused: %+v posts=%d", action, res, posts())
			}
		})
	}
}

func TestCreateOrderElicitationShowsDefaults(t *testing.T) {
	srv, _, _ := twoStepServer(t)
	_, first := callElicit(t, srv, "tools/call", orderCall(nil), formCaps)
	ir, _ := json.Marshal(first["inputRequests"])
	if !strings.Contains(string(ir), "status bosatt") || !strings.Contains(string(ir), "alder 30") {
		t.Fatalf("confirmation form must show the defaults: %s", ir)
	}
}
