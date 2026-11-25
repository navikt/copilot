package policy

import "strings"

// Risk ranks a tool's effect: read < propose < write (#2964).
type Risk int

// Risk levels in rank order.
const (
	RiskRead Risk = iota
	RiskPropose
	RiskWrite
)

func (r Risk) String() string {
	switch r {
	case RiskRead:
		return "read"
	case RiskPropose:
		return "propose"
	default:
		return "write"
	}
}

// ParseRisk maps a snapshot risk value to its rank. Unknown or empty is write,
// so a missing classification never ranks a tool lower than it may be.
func ParseRisk(s string) Risk {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "read":
		return RiskRead
	case "propose":
		return RiskPropose
	default:
		return RiskWrite
	}
}

// ToolRisk is the risk of tool on a route whose snapshot gave toolRisk.
func ToolRisk(toolRisk map[string]string, tool string) Risk {
	return ParseRisk(toolRisk[tool])
}

// Caller is who calls a tool. Logged-in humans may use every risk level on
// routes they reach; only agent profiles are capped by MaxRisk.
type Caller struct {
	Agent   bool
	MaxRisk Risk
}

// Kind is the caller_kind metric label.
func (c Caller) Kind() string {
	if c.Agent {
		return "agent"
	}
	return "human"
}

// RiskAllowed reports whether c may call a tool of risk r.
func RiskAllowed(c Caller, r Risk) bool {
	return !c.Agent || r <= c.MaxRisk
}

// CheckRisk decides a tools/call by risk. decision is allow or would_deny;
// refuse is true only when enforce is set, so report mode never refuses.
func CheckRisk(toolRisk map[string]string, tool string, c Caller, enforce bool) (r Risk, decision string, refuse bool) {
	r = ToolRisk(toolRisk, tool)
	if RiskAllowed(c, r) {
		return r, "allow", false
	}
	return r, "would_deny", enforce
}
