package policy

import "testing"

func TestParseRiskUnknownIsWrite(t *testing.T) {
	for in, want := range map[string]Risk{"read": RiskRead, "Propose": RiskPropose, "write": RiskWrite, "": RiskWrite, "admin": RiskWrite} {
		if got := ParseRisk(in); got != want {
			t.Errorf("ParseRisk(%q) = %v, want %v", in, got, want)
		}
	}
	if got := ToolRisk(nil, "x"); got != RiskWrite {
		t.Errorf("missing tool_risk = %v, want write", got)
	}
}

func TestRiskAllowed(t *testing.T) {
	agentRead := Caller{Agent: true, MaxRisk: RiskRead}
	if RiskAllowed(agentRead, RiskWrite) || RiskAllowed(agentRead, RiskPropose) {
		t.Error("agent with max_risk=read must not call propose or write")
	}
	if !RiskAllowed(agentRead, RiskRead) {
		t.Error("agent with max_risk=read must call read")
	}
	if !RiskAllowed(Caller{}, RiskWrite) {
		t.Error("a human is never refused on risk")
	}
}

func TestCheckRisk(t *testing.T) {
	agentRead := Caller{Agent: true, MaxRisk: RiskRead}
	risks := map[string]string{"get": "read", "put": "write"}
	if _, d, refuse := CheckRisk(risks, "put", agentRead, false); refuse || d != "would_deny" {
		t.Errorf("report mode: decision %q refuse %v, want would_deny without refusal", d, refuse)
	}
	if r, _, refuse := CheckRisk(risks, "put", agentRead, true); !refuse || r != RiskWrite {
		t.Error("enforce must refuse an agent with max_risk=read calling write")
	}
	if r, _, refuse := CheckRisk(risks, "unclassified", agentRead, true); !refuse || r != RiskWrite {
		t.Error("unknown risk must count as write")
	}
	if _, _, refuse := CheckRisk(risks, "get", agentRead, true); refuse {
		t.Error("read within max_risk must pass")
	}
	if _, d, refuse := CheckRisk(risks, "put", Caller{}, true); refuse || d != "allow" {
		t.Error("a human is never refused")
	}
}
