package upstream

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/navikt/copilot/apps/copilot-cli/internal/texas"
)

// Texas down or copilot-api unreachable: refused too.
func TestMembershipUnreachable(t *testing.T) {
	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close()
	for _, u := range []*Upstream{
		New("copilot-api", dead.URL, texas.New(dead.URL+"/token", "api://x")),
		New("copilot-api", dead.URL, texas.New("", "api://x")),
	} {
		if ok, err := u.IsOrgMember(t.Context(), "hans"); ok || err == nil {
			t.Fatalf("ok=%v err=%v, want an error", ok, err)
		}
	}
}
