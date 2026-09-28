package provider

import (
	"net"
	"net/url"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
	"github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// ActionCheckServerEnv hands the action check (`nav-pilot hook action-check`)
// the local model server to ask: "<base URL> <model id>", with " endpoint"
// after them for the developer's own server (local_endpoint), which decide asks
// differently (reasoning_effort "none"). The hook runs inside
// cplt, which denies ~/.nav-pilot, so it cannot look the server up itself
// (#1165). The launch looks it up outside the sandbox and passes it here.
const ActionCheckServerEnv = "NAV_PILOT_ACTION_CHECK_SERVER"

// actionCheckServer is the server the action check asks this session, or ""
// when the check is off or no server is running at launch. A server started
// later is not seen until the next launch; the hook skips until then.
//
// ponytail: the managed server is proven ours here, at launch, not on every
// call: the hook cannot see its pid inside cplt. A server that dies mid-session
// and leaves its port to another local process would get the check's prompts
// (a command and its stated purpose) until the session ends. Re-prove per call
// if the hook ever gets a way to do that from inside the sandbox.
func actionCheckServer(r domain.ResolvedConfig) (base, model string, endpoint bool) {
	if r.HookActionCheck == "off" || !r.LocalEnabled {
		return "", "", false
	}
	if base, model = local.Endpoint(); base != "" {
		return base, model, true
	}
	if local.EnsureOwnServer() != nil {
		return "", "", false
	}
	st, ok, err := local.LoadState()
	if err != nil || !ok {
		return "", "", false
	}
	return local.ServerURL(), st.Model, false
}

// withActionCheckServer sets ActionCheckServerEnv for the session and returns
// the cplt flags that let it and the server's port through the sandbox: none
// when there is no server, and no port for a server not on loopback.
func withActionCheckServer(r domain.ResolvedConfig, env []string) ([]string, []string) {
	base, model, endpoint := actionCheckServer(r)
	if base == "" {
		return env, nil
	}
	value := base + " " + model
	if endpoint {
		value += " endpoint"
	}
	env, _ = telemetry.SetEnvValue(env, ActionCheckServerEnv, value)
	flags := []string{"--pass-env", ActionCheckServerEnv}
	if u, err := url.Parse(base); err == nil {
		if ip := net.ParseIP(u.Hostname()); (ip != nil && ip.IsLoopback()) || u.Hostname() == "localhost" {
			if port := u.Port(); port != "" {
				flags = append(flags, "--allow-localhost", port)
			}
		}
	}
	return env, flags
}
