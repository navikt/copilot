package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// CopilotSession is what one Copilot CLI session did, read from its
// events.jsonl when it ends. The caller has already mapped every name to an
// allowlist or "other" and every model to a known id or "custom": this type
// carries counts and bounded labels only, never content.
type CopilotSession struct {
	Client string
	Skills map[SkillKey]int64
	Tools  map[ToolKey]int64
	Hooks  map[HookKey]int64
	Agents map[string]int64
	Models map[string]SessionModel
	Turns  int64
	// DurationSeconds is negative when the session start is unknown.
	DurationSeconds int64
	ShutdownType    string
	APIDurationMS   int64
	APIModel        string
	// Skipped is lines that were not valid JSON or had an unexpected shape.
	Skipped int64
}

type SkillKey struct{ Skill, Source, Trigger string }
type ToolKey struct{ Tool, Outcome string }
type HookKey struct{ HookType, Outcome string }

type SessionModel struct {
	Input, Output, CacheRead, CacheWrite int64
	PremiumRequests                      float64
	NanoAIU                              int64
}

type sessionInstruments struct {
	skills, tools, hooks, tokens, nanoAIU, agents, skipped metric.Int64Counter
	premium                                                metric.Float64Counter
	turns, duration, apiDuration                           metric.Int64Histogram
}

func newSessionInstruments(meter metric.Meter) (sessionInstruments, error) {
	var s sessionInstruments
	var errs []error
	counter := func(name, desc string) metric.Int64Counter {
		c, err := meter.Int64Counter(name, metric.WithDescription(desc))
		errs = append(errs, err)
		return c
	}
	hist := func(name, desc string, bounds ...float64) metric.Int64Histogram {
		h, err := meter.Int64Histogram(name, metric.WithDescription(desc), metric.WithExplicitBucketBoundaries(bounds...))
		errs = append(errs, err)
		return h
	}
	s.skills = counter("nav_pilot_session_skill_loads_total", "Skills a Copilot session loaded, by skill (Nav allowlist or other), source and trigger.")
	s.tools = counter("nav_pilot_session_tool_calls_total", "Tool calls in a Copilot session, by tool (built-in or other) and outcome.")
	s.hooks = counter("nav_pilot_session_hook_runs_total", "Hook runs in a Copilot session, by hook type and outcome.")
	s.tokens = counter("nav_pilot_session_tokens_total", "Tokens a Copilot session used, by model and kind.")
	s.nanoAIU = counter("nav_pilot_session_cost_nano_aiu_total", "Cost of a Copilot session in nano AIU, by model.")
	s.agents = counter("nav_pilot_subagent_selected_total", "Subagents a Copilot session selected, by agent (Nav allowlist or other).")
	s.skipped = counter("nav_pilot_session_events_skipped_total", "events.jsonl lines nav-pilot could not parse and skipped.")
	var err error
	s.premium, err = meter.Float64Counter("nav_pilot_session_premium_requests_total", metric.WithDescription("Premium requests a Copilot session used, by model."))
	errs = append(errs, err)
	s.turns = hist("nav_pilot_session_turns", "Assistant turns in a Copilot session.", 1, 2, 5, 10, 20, 50, 100, 200, 500)
	s.duration = hist("nav_pilot_session_duration_seconds", "Wall-clock length of a Copilot session.", 60, 300, 900, 1800, 3600, 7200, 14400, 28800, 86400)
	s.apiDuration = hist("nav_pilot_session_api_duration_ms", "Time a Copilot session spent waiting on the model API.", 1000, 5000, 15000, 60000, 300000, 900000, 3600000)
	for _, e := range errs {
		if e != nil {
			return s, fmt.Errorf("create session instruments: %w", e)
		}
	}
	return s, nil
}

// RecordCopilotSession adds one session's totals. Like RecordClientUsage it
// carries no per-session or per-device label: each export is what is new, so
// sum_over_time adds the sessions up.
func (t *otelTelemetry) RecordCopilotSession(s CopilotSession) {
	ctx := context.Background()
	in := t.session
	with := func(kv ...attribute.KeyValue) metric.MeasurementOption {
		return metric.WithAttributes(append(kv,
			attribute.String("client", orUnset(s.Client)),
			attribute.String("version", t.version))...)
	}
	str := attribute.String
	for k, n := range s.Skills {
		in.skills.Add(ctx, n, with(str("skill", k.Skill), str("source", k.Source), str("trigger", k.Trigger)))
	}
	for k, n := range s.Tools {
		in.tools.Add(ctx, n, with(str("tool", k.Tool), str("outcome", k.Outcome)))
	}
	for k, n := range s.Hooks {
		in.hooks.Add(ctx, n, with(str("hook_type", k.HookType), str("outcome", k.Outcome)))
	}
	for a, n := range s.Agents {
		in.agents.Add(ctx, n, with(str("agent", a)))
	}
	for model, m := range s.Models {
		for kind, n := range map[string]int64{"input": m.Input, "output": m.Output, "cache_read": m.CacheRead, "cache_write": m.CacheWrite} {
			if n > 0 {
				in.tokens.Add(ctx, n, with(str("model", model), str("kind", kind)))
			}
		}
		if m.PremiumRequests > 0 {
			in.premium.Add(ctx, m.PremiumRequests, with(str("model", model)))
		}
		if m.NanoAIU > 0 {
			in.nanoAIU.Add(ctx, m.NanoAIU, with(str("model", model)))
		}
	}
	in.turns.Record(ctx, s.Turns, with())
	if s.DurationSeconds >= 0 {
		in.duration.Record(ctx, s.DurationSeconds, with(str("shutdown_type", orUnset(s.ShutdownType))))
	}
	if s.APIDurationMS > 0 {
		in.apiDuration.Record(ctx, s.APIDurationMS, with(str("model", orUnset(s.APIModel))))
	}
	if s.Skipped > 0 {
		in.skipped.Add(ctx, s.Skipped, with())
	}
}
