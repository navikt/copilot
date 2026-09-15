package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// naisdevice readiness, reported for the gate rather than for naisdevice.
//
// nais/pilot ships a preToolUse gate that refuses cluster commands aimed at the
// wrong tenant. The gate can only do that if it can see which tenant naisdevice
// has active. Inside cplt it usually cannot: `nais device …` talks gRPC over
// ~/Library/Application Support/naisdevice/agent.sock, and the sandbox denies
// connect() to sockets it was not given (docs/README.nais-cli.md). The failure
// is silent and it looks exactly like naisdevice not running, so a developer
// with an ungated session has no way to find out. This section is that way.
//
// Everything here is warn-only and sets no error state. nav-pilot neither
// installs naisdevice nor owns the tenant, and a disconnected VPN is a normal
// thing to be on a Friday afternoon.

// naisDeviceTimeout bounds the agent lookup. A wedged naisdevice agent has been
// measured taking over 20 seconds to answer, and doctor must not hang on it:
// the question is worth three seconds and not one more. Its own deadline, like
// every other spawn in doctor.
const naisDeviceTimeout = 3 * time.Second

// errNaisDeviceUnreachable is what a failed lookup returns. The error from the
// command itself is deliberately dropped: exec's ExitError carries the process
// stderr, `nais device status` is a command whose output holds a bearer token,
// and an error value that gets printed is an output path like any other.
var errNaisDeviceUnreachable = errors.New("naisdevice agent did not answer")

// naisTenantAliases are the two tenants whose gate name is not just their
// domain with the TLD cut off. Mirrors the normalisation in nais/pilot's gate;
// if that list grows, this one follows.
var naisTenantAliases = map[string]string{
	"arbeidstilsynet":       "atil",
	"landbruksdirektoratet": "ldir",
}

// gateTenant turns the tenant name naisdevice reports — a domain, "NAV",
// "dev-nais.io", "ssb.no" — into the short form nais/pilot's gate matches on.
// doctor shows both: the user recognises the domain, the gate logs the alias,
// and a user comparing the two needs to be told they are the same thing.
func gateTenant(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.TrimSuffix(s, ".no")
	s = strings.TrimSuffix(s, ".io")
	if alias, ok := naisTenantAliases[s]; ok {
		return alias
	}
	return s
}

// naisDeviceStatus asks the local naisdevice agent for connection state and
// active tenant.
//
// The token discipline is the decode itself. `nais device status --output json`
// reports AgentStatus with a session bearer token at Tenants[].session.key, and
// the struct below has no field it can land in, so encoding/json drops it at
// the boundary: no value in this process ever holds it, which is a stronger
// guarantee than remembering not to print one. The raw bytes stay local to this
// function and are never returned, logged or wrapped into an error.
//
// A package var so tests can stub the machine away.
var naisDeviceStatus = func(naisPath string) (connected bool, tenant string, err error) {
	out, err := runBoundedTimeout(naisDeviceTimeout, false, naisPath, "device", "status", "--output", "json")
	if err != nil {
		return false, "", errNaisDeviceUnreachable
	}
	return parseNaisDeviceStatus(out)
}

// parseNaisDeviceStatus reads connection state and the active tenant out of the
// agent's JSON. The field tags are lowercase and encoding/json matches field
// names case-insensitively, so this reads both the Go-exported spelling
// ("Tenants", "Name") and the wire spelling without a second set of tags.
func parseNaisDeviceStatus(out []byte) (connected bool, tenant string, err error) {
	var doc struct {
		ConnectionState string `json:"connectionState"`
		Tenants         []struct {
			Name   string `json:"name"`
			Active bool   `json:"active"`
		} `json:"tenants"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		// Not the parse error: it quotes the input it choked on.
		return false, "", errNaisDeviceUnreachable
	}
	for _, t := range doc.Tenants {
		if t.Active {
			tenant = t.Name
			break
		}
	}
	return strings.EqualFold(doc.ConnectionState, "connected"), tenant, nil
}

// naisAgentStatus is the file naisdevice writes beside its config from
// nais/device#564, and the only way the gate sees tenant state from inside a
// sandbox that denies the agent socket.
type naisAgentStatus struct {
	ConnectionState  string    `json:"connectionState"`
	Tenant           string    `json:"tenant"`
	UpdatedAt        time.Time `json:"updatedAt"`
	HeartbeatSeconds int       `json:"heartbeatSeconds"`
	Warning          string    `json:"warning"`
}

// naisStatusFilePath is where naisdevice keeps it: ~/Library/Application
// Support/naisdevice on macOS, $XDG_CONFIG_HOME/naisdevice or ~/.config/
// naisdevice elsewhere. os.UserConfigDir already resolves exactly that split.
func naisStatusFilePath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "naisdevice", "agent-status.json")
}

// stale reports whether the file has stopped being refreshed. Three missed
// heartbeats, so an ordinary late write is not called a fault; a status file
// that stopped updating is worse than none, because the gate would enforce
// against a tenant the user left hours ago.
func (s naisAgentStatus) stale(now time.Time) bool {
	beat := time.Duration(s.HeartbeatSeconds) * time.Second
	if beat <= 0 {
		beat = 30 * time.Second
	}
	return now.Sub(s.UpdatedAt) > 3*beat
}

// readNaisAgentStatus reads the status file. A missing file is the normal case
// today and returns (nil, nil): nais/device#564 is unreleased, so no machine
// has one yet.
func readNaisAgentStatus(path string) (*naisAgentStatus, error) {
	if path == "" {
		return nil, os.ErrNotExist
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s naisAgentStatus
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("unreadable status file")
	}
	return &s, nil
}

// naisPakkeInstalled reports whether any installed agentpakke comes from the
// nais org.
//
// This is the condition for showing the section at all. A developer running a
// pakke from somewhere else has no tenant gate, and telling them about tenants
// would be doctor inventing a problem. Keying on the source org rather than on
// the pakke name is the loose end of the two on purpose: the gate lives in
// nais/pilot today, but any nais-owned pakke is entitled to ship one, and being
// told about naisdevice one pakke too early is a much smaller failure than the
// silent ungated session this section exists to catch.
func naisPakkeInstalled(states ...*StateFile) bool {
	for _, s := range states {
		if s != nil && strings.HasPrefix(s.SourceRepo, "nais/") {
			return true
		}
	}
	return false
}

// reportNaisdevice is the doctor section. naisPath is empty when `nais` is not
// on PATH; statusPath is where the status file would be.
func reportNaisdevice(w io.Writer, naisPath, statusPath string) {
	if naisPath == "" {
		fmt.Fprintf(w, "    %s naisdevice not found: no %s on PATH\n", yellow("⚠"), bold("nais"))
		fmt.Fprintf(w, "        The tenant gate in your nais agentpakke has nothing to read, so cluster\n")
		fmt.Fprintf(w, "        commands are not being checked against a tenant at all.\n")
		fmt.Fprintf(w, "        %s Install naisdevice: %s\n", yellow("Solution:"), bold("https://doc.nais.io/operate/naisdevice/how-to/install/"))
		return
	}
	fmt.Fprintf(w, "    %s Binary found: %s\n", green("✓"), naisPath)

	connected, tenant, err := naisDeviceStatus(naisPath)
	switch {
	case err != nil && insideCpltSandbox():
		// The expected answer in here, and the one that reads as a fault if
		// nobody says otherwise: `nais device status` returns "make sure
		// naisdevice is running" whether or not it is running, because cplt
		// denied connect() to the agent socket long before naisdevice saw it.
		fmt.Fprintf(w, "    %s Cannot ask the agent from inside cplt — the sandbox denies its socket\n", dim("-"))
		fmt.Fprintf(w, "        This says nothing about naisdevice; the same answer comes back when it is\n")
		fmt.Fprintf(w, "        connected. The status file below is what the gate reads in here.\n")
	case err != nil:
		fmt.Fprintf(w, "    %s naisdevice did not answer within %s\n", yellow("⚠"), naisDeviceTimeout)
		fmt.Fprintf(w, "        %s Check that naisdevice is running, then re-run %s.\n", yellow("Solution:"), bold("nav-pilot doctor"))
	case !connected:
		fmt.Fprintf(w, "    %s naisdevice is installed but not connected\n", yellow("⚠"))
		fmt.Fprintf(w, "        Without a tenant the gate cannot tell your clusters from another tenant's.\n")
		fmt.Fprintf(w, "        %s Connect naisdevice, then re-run %s.\n", yellow("Solution:"), bold("nav-pilot doctor"))
	case tenant == "":
		fmt.Fprintf(w, "    %s Connected, but no tenant is marked active\n", yellow("⚠"))
		fmt.Fprintf(w, "        %s Pick a tenant in naisdevice.\n", yellow("Solution:"))
	default:
		fmt.Fprintf(w, "    %s Connected to %s (the gate calls this tenant %s)\n",
			green("✓"), bold(tenant), bold(gateTenant(tenant)))
	}

	reportNaisStatusFile(w, statusPath)
}

// reportNaisStatusFile is the half that actually matters: whether the gate can
// see any of the above from inside the sandbox.
func reportNaisStatusFile(w io.Writer, statusPath string) {
	status, err := readNaisAgentStatus(statusPath)
	switch {
	case err != nil:
		fmt.Fprintf(w, "    %s Status file is there but unreadable: %s\n", yellow("⚠"), statusPath)
		fmt.Fprintf(w, "        The gate reads this file inside cplt, so it is not enforcing tenant rules there.\n")
		return
	case status == nil:
		// The common case, on every machine, today. Say so without implying
		// the user broke something.
		fmt.Fprintf(w, "    %s No status file yet (%s)\n", dim("-"), statusPath)
		fmt.Fprintf(w, "        naisdevice writes one from nais/device#564, which is not released. Until your\n")
		fmt.Fprintf(w, "        naisdevice is new enough, the gate cannot see your tenant inside cplt and does\n")
		fmt.Fprintf(w, "        not enforce tenant rules there. Nothing on this machine is misconfigured.\n")
		return
	case status.stale(time.Now()):
		fmt.Fprintf(w, "    %s Status file has not been updated since %s (%s)\n",
			yellow("⚠"), status.UpdatedAt.Format(time.RFC3339), statusPath)
		fmt.Fprintf(w, "        The gate would enforce against whatever this file still says. Check that\n")
		fmt.Fprintf(w, "        naisdevice is running; the file refreshes on its own once it is.\n")
		return
	}

	if status.Warning != "" {
		fmt.Fprintf(w, "    %s naisdevice says: %s\n", yellow("⚠"), status.Warning)
	}
	if status.Tenant != "" {
		fmt.Fprintf(w, "    %s Status file is current: %s on %s\n",
			green("✓"), bold(status.Tenant), status.ConnectionState)
	} else {
		fmt.Fprintf(w, "    %s Status file is current: %s\n", green("✓"), status.ConnectionState)
	}

	if insideCpltSandbox() {
		// Read from inside the sandbox, which is the proof rather than the
		// inference: the gate runs here and can read what this just read.
		fmt.Fprintf(w, "        %s The gate can read this from inside cplt, so tenant rules are enforced.\n", green("✓"))
		return
	}
	fmt.Fprintf(w, "        %s cplt denies this file unless it is granted. Without the grant the gate\n", yellow("Note:"))
	fmt.Fprintf(w, "        cannot see your tenant in a session and does not enforce tenant rules.\n")
	fmt.Fprintf(w, "        %s %s\n", yellow("Solution:"), bold(fmt.Sprintf("cplt config set allow.read %q", statusPath)))
}
