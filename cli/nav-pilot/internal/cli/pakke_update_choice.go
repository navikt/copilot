package cli

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// The durable update choice of a pinned agentpakke (#781).
//
// A user who installed an agentpakke decides once what happens when the package
// publishes a newer stable release: take it automatically, be asked first, or
// keep the revision. The answer is not about one release, so it does not live in
// the release cache with the lookups: it lives in the scope's own state, next to
// the pin it is about, and is carried across every pin that scope writes. An
// `uninstall` takes it with the pin, which is right — it is a choice about an
// installed package, and there is none afterwards.
//
// It is deliberately not an extension of `auto_update`. That key is about the
// nav-pilot binary and is machine-wide; this one is per installed package, and a
// user who wants the binary to keep itself current has said nothing about
// whether someone else's agentpakke may move under them.
//
// Three answers, and no fourth:
//
//   - "auto" moves the pin at startup in a terminal, without a question.
//   - "ask" is the default, and what every state written before this field
//     reads as: the startup prompt asks once per release, and "Later" is
//     remembered for that version in the release cache.
//   - "keep" stops both. No lookup at startup, no question, and no automatic
//     move — but not a blindfold: `sync` and `status` still name a newer
//     release and the command that takes it, so a security fix is never
//     hidden from the person who chose to sit still. Nothing about "keep"
//     makes nav-pilot pin a revision the user did not ask for, security fix or
//     not; the publisher's fix arrives when the user says so, which is the
//     whole meaning of the choice.
//
// The choice never overrides an explicit command. `install`, `sync --apply
// --ref <sha>` and `rollback` are the user acting now, and they act.
type updateChoice string

const (
	// updateAsk is the default, and what an empty field reads as.
	updateAsk  updateChoice = "ask"
	updateAuto updateChoice = "auto"
	updateKeep updateChoice = "keep"
)

// updateChoices lists the choices in the order they are offered and documented.
var updateChoices = []updateChoice{updateAuto, updateAsk, updateKeep}

// label is the choice in a status line.
func (c updateChoice) label() string {
	switch c {
	case updateAuto:
		return "automatic"
	case updateKeep:
		return "keep this revision"
	default:
		return "ask first"
	}
}

// parseUpdateChoice accepts exactly the three values, and nothing else: a
// mistyped mode must not read as a mode that silently does less.
func parseUpdateChoice(s string) (updateChoice, bool) {
	for _, c := range updateChoices {
		if s == string(c) {
			return c, true
		}
	}
	return "", false
}

// pakkeUpdateChoice is the scope's choice, defaulting to ask.
func pakkeUpdateChoice(state *StateFile) updateChoice {
	if state == nil {
		return updateAsk
	}
	c, ok := parseUpdateChoice(state.UpdateChoice)
	if !ok {
		// A value a newer nav-pilot wrote, or a hand-edited state. Asking is
		// the answer that neither moves a pin nor hides an update.
		return updateAsk
	}
	return c
}

// updateHold is why this scope's pin may not move onto a revision on nav-pilot's
// own initiative. It is one predicate on purpose: two durable records say "not
// this", and answering them separately is how two checks start disagreeing.
// [StateFile.RolledBackFrom] names one revision this scope rejected (#783), and
// updateKeep refuses every revision newer than the pin (#781). A caller asks
// once, about the revision it is holding, and gets one answer.
type updateHold string

const (
	holdNone       updateHold = ""
	holdRolledBack updateHold = "rolled_back"
	holdKeep       updateHold = "keep"
)

// pakkeUpdateHold answers for sha: holdNone when the pin may move onto it.
//
// Re-pinning the revision already pinned is never a move, so it is never held:
// that is how a scope whose revision directory was wiped is restored at its own
// SHA while both records still say what they say.
func pakkeUpdateHold(state *StateFile, sha string) updateHold {
	switch {
	case state == nil || sha == "" || sameSHA(sha, state.SourceSHA):
		return holdNone
	case sameSHA(sha, state.RolledBackFrom):
		return holdRolledBack
	case pakkeUpdateChoice(state) == updateKeep:
		return holdKeep
	}
	return holdNone
}

// why names the held revision in a sentence about it, for the one message sync
// prints whichever record held it back.
func (h updateHold) why(state *StateFile) string {
	if h == holdRolledBack {
		return "the one this scope was rolled back from"
	}
	return "newer than " + shortSHA(state.SourceSHA) + ", which this scope's update choice keeps"
}

// stateWriteHook is a test seam: when non-nil it runs in [setUpdateChoice] just
// before the read the write is built on, which is the window a concurrent pin
// lands in.
var stateWriteHook func()

// setUpdateChoice writes choice into this scope's state.
//
// It re-reads immediately before writing and sets the field on what it finds,
// rather than on the state its caller read a moment ago. That is the lost-update
// guard [pinRevision] has, applied to a write that has nothing to refuse: a pin
// that moved while the choice was being made is the pin now, and the choice is
// one independent field, so merging onto the newer state is the whole answer
// where pinRevision has to refuse. Writing the caller's snapshot instead would
// restore the SourceSHA a concurrent launch, sync or rollback had just moved,
// along with every file record written with it.
//
// ponytail: the window is narrowed to the read and the write, not closed — there
// is no lock over the state file, and every other writer here has the same
// window. Add one for all of them, not for this caller alone, if it ever bites.
func setUpdateChoice(scope *InstallScope, choice updateChoice) error {
	if stateWriteHook != nil {
		stateWriteHook()
	}
	state, err := readScopedState(scope)
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}
	if state == nil {
		return fmt.Errorf("the %s scope's installation was removed while the choice was being made; nothing was recorded", scope.Name)
	}
	state.UpdateChoice = string(choice)
	if err := writeScopedState(scope, state); err != nil {
		return fmt.Errorf("writing state: %w", err)
	}
	return nil
}

// cmdUpdateChoice records the choice for the user scope's pinned agentpakke.
// `sync --updates <mode>` runs it before the sync itself, so setting the choice
// and acting on it is one command: `sync --apply --updates auto` says "take new
// releases from now on, this one included".
func cmdUpdateChoice(value string, jsonOutput bool) error {
	choice, ok := parseUpdateChoice(value)
	if !ok {
		var names []string
		for _, c := range updateChoices {
			names = append(names, string(c))
		}
		return fmt.Errorf("--updates takes %s, not %q.\n\n"+
			"  %s  take every new stable release at startup\n"+
			"  %s  ask before a new stable release is pinned (the default)\n"+
			"  %s  keep the pinned revision and stop asking",
			strings.Join(names, ", "), value,
			bold("auto"), bold("ask "), bold("keep"))
	}
	scope, err := ScopeUser()
	if err != nil {
		return fmt.Errorf("locating your user scope: %w", err)
	}
	state, err := readScopedState(scope)
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}
	if !pinnedState(state) || !pinnable(state.SourceRepo) {
		// Say what the user does have, since that is what they meant to
		// change (#16): files in ~/.copilot, or a repository's lock file.
		var have []string
		if state != nil && installsContent(state) {
			have = append(have, fmt.Sprintf("Your user scope holds files from %s, and %s updates them.",
				cmp.Or(state.SourceRepo, "an agentpakke"), bold("nav-pilot sync --user --apply")))
		}
		if root := findGitRoot("."); root != "" {
			if d, _ := scopeDeclaration(ScopeRepo(root)); d != nil && d.SHA != "" {
				have = append(have, fmt.Sprintf("This repository pins %s at %s in %s. %s moves the pin, and %s moves it to a revision you choose.",
					d.Source, shortSHA(d.SHA), agentpakke.DeclarationPath, bold("nav-pilot sync --repo --apply"), bold("--ref")))
			}
		}
		if len(have) > 0 {
			return fmt.Errorf("--updates sets how an agentpakke pinned as a revision in your user scope takes new stable releases, and your user scope pins none.\n\n%s",
				strings.Join(have, "\n"))
		}
		return fmt.Errorf(
			"--updates sets what happens when a pinned agentpakke publishes a new stable release, and your user scope pins none.\n\n"+
				"  Pin one:  %s",
			bold("nav-pilot install --user <name>"))
	}
	if err := setUpdateChoice(scope, choice); err != nil {
		return err
	}
	// Read back rather than reporting the snapshot above: the pin named here is
	// the one the choice was written onto, which a concurrent pin may have moved.
	if written, err := readScopedState(scope); err == nil && written != nil {
		state = written
	}
	if jsonOutput {
		return nil // the sync's own document follows on stdout
	}
	switch choice {
	case updateAuto:
		fmt.Printf("%s %s takes new stable releases at startup.\n", green("✓"), bold(state.Collection))
	case updateKeep:
		fmt.Printf("%s %s keeps revision %s. New stable releases are reported, never pinned.\n",
			green("✓"), bold(state.Collection), shortSHA(state.SourceSHA))
	default:
		fmt.Printf("%s %s asks at startup before a new stable release is pinned.\n", green("✓"), bold(state.Collection))
	}
	return nil
}
