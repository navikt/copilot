package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// closedServerURL is the address of an httptest server that has shut down, so
// a request to it fails the way an unreachable host does.
func closedServerURL() string {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	return srv.URL
}

// Offline, nav-pilot news and nav-pilot mcp list say what they could not reach
// and what to do, not Go's raw network error (#2906).
func TestOfflineSaysWhatToDo(t *testing.T) {
	isolatedConfig(t)
	t.Setenv("NAV_PILOT_NEWS_URL", closedServerURL())
	err := cmdNews(false)
	if err == nil || err.Error() != "fikk ikke kontakt med ki-utvikling.nav.no. Sjekk nettverket, og prøv igjen" {
		t.Errorf("news: %v", err)
	}

	mcpCmdEnv(t, providerpkg.MCPHostState{})
	url := closedServerURL()
	mcpRegistryServers = func() (string, []providerpkg.MCPServerEntry, error, error) {
		_, err := http.Get(url)
		return "", nil, nil, err
	}
	err = cmdMCP([]string{"list"})
	if err == nil || strings.Contains(err.Error(), "connect") || !strings.Contains(err.Error(), "Check the network or naisdevice") {
		t.Errorf("mcp list: %v", err)
	}
	for _, args := range [][]string{{"enable", "x"}, {"prune"}} {
		err = cmdMCP(args)
		if err == nil || strings.Contains(err.Error(), "connect") || !strings.Contains(err.Error(), "nothing was changed. Check the network or naisdevice") {
			t.Errorf("mcp %s: %v", args[0], err)
		}
	}
}
