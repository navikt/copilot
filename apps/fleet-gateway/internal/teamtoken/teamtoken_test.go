package teamtoken

import (
	"errors"
	"testing"
)

func TestEntraScopeForTarget(t *testing.T) {
	if got := entraScopeForTarget("dev-gcp:copilot:fleet-echo"); got != "api://dev-gcp.copilot.fleet-echo/.default" {
		t.Fatalf("entraScopeForTarget = %q, want %q", got, "api://dev-gcp.copilot.fleet-echo/.default")
	}
}

// Entra refusals caused by the target app map to TargetAppError; unknown codes stay generic (#3105).
func TestMapUpstreamTokenFailureTargetApp(t *testing.T) {
	body := func(c string) []byte {
		return []byte(`{"error":"invalid_resource","error_description":"` + c + `: nope"}`)
	}
	var ta *TargetAppError
	var am *AccessMissingError
	for _, c := range []string{"AADSTS500011", "AADSTS65005"} {
		if err := mapUpstreamTokenFailure(body(c), "app", 400); !errors.As(err, &ta) || ta.Code != c {
			t.Errorf("%s: got %v, want TargetAppError", c, err)
		}
	}
	if err := mapUpstreamTokenFailure(body("AADSTS65001"), "app", 400); !errors.As(err, &am) {
		t.Errorf("AADSTS65001: got %v, want AccessMissingError", err)
	}
	for _, c := range []string{"AADSTS700016", "AADSTS5000110", "AADSTS70011"} {
		err := mapUpstreamTokenFailure(body(c), "app", 400)
		if errors.As(err, &ta) || errors.As(err, &am) {
			t.Errorf("%s: got %T, want generic upstream error", c, err)
		}
	}
}
