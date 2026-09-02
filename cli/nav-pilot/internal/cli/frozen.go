package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// `install --frozen` is the CI and onboarding counterpart to the committed
// declaration. Install already *honours* the pin; what --frozen adds is a
// refusal to paper anything over: it never prompts, never moves the pin, and
// never reports success on an install that did not fully land.
//
// The guiding question for every case below is what a CI job would rather fail
// on than accept. A job that installs from an unpinned repo installs whatever
// the default branch happened to be that morning; a job that installs half the
// agents and exits 0 is the exact failure the pin exists to prevent.
//
// Exit codes are the surface CI actually reads:
//
//	1 (ExitError)  the install failed — source unreachable, bad flags, a broken
//	               agentpakke. Something to fix outside the pin.
//	3 (ExitFrozen) the install did not fail, but the repository's declared pin
//	               was not honoured: nothing declared, nothing pinned, an
//	               unusable pin, a revision other than the declared one, or a
//	               partial install. Nothing was proven about the pin, which is
//	               deliberately not 0 — the same stance scripts/nav-pilot-golden.sh
//	               takes with its exit 3.
//
// installFrozen carries the flag for the lifetime of one invocation. The flag
// reaches five call sites across two files that already take four booleans
// each; run() sets it once and clears it, and the tests set it the same way.
var installFrozen bool

// frozenError is a refusal by --frozen, mapped to ExitFrozen by exitCodeFor.
type frozenError struct{ error }

func frozenf(format string, a ...interface{}) error {
	return frozenError{fmt.Errorf(format, a...)}
}

func isFrozenRefusal(err error) bool {
	var fe frozenError
	return errors.As(err, &fe)
}

// isFullSHA reports whether s is a fetchable pin. git refuses an abbreviated
// object id in a fetch request, so a short sha is a pin nobody can install
// back (#607) — and under --frozen it must say that rather than surface git's
// own message about a ref that does not exist.
func isFullSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

// frozenPrecheck refuses, before anything is resolved or written, every repo
// state a frozen install cannot be frozen against.
func frozenPrecheck(scope *InstallScope) error {
	if !installFrozen {
		return nil
	}
	if scope == nil || scope.IsUser() {
		return frozenf("--frozen needs a repository: user scope never reads %s, so there is no pin to hold",
			agentpakke.DeclarationPath)
	}
	d, err := scopeDeclaration(scope)
	if err != nil {
		// A declaration that will not load is a pin nobody is honouring, which
		// is this flag's own failure mode rather than an ordinary error.
		return frozenError{err}
	}
	if d == nil {
		return frozenf("--frozen needs a declared agentpakke, and this repository has no %s.\n"+
			"Run %s once, review the file it writes, and commit it — then CI can install exactly that revision",
			agentpakke.DeclarationPath, bold("nav-pilot install <name>"))
	}
	sha := strings.TrimSpace(d.SHA)
	if sha == "" {
		return frozenf("%s names %s but pins no revision, so a frozen install would take whatever the default branch is today.\n"+
			"Run %s and commit the pin it writes",
			agentpakke.DeclarationPath, bold(d.Source), bold("nav-pilot install <name>"))
	}
	if !isFullSHA(sha) {
		return frozenf("%s pins %q, which is not a full 40-character commit SHA.\n"+
			"git refuses an abbreviated object id in a fetch, so that pin cannot be installed back. Write the whole SHA",
			agentpakke.DeclarationPath, sha)
	}
	return nil
}

// frozenPinMatches refuses a resolved source that is not the declared
// revision. Under --frozen every route to another revision is already refused,
// so this is the assertion that keeps it that way: it is the difference
// between "the install failed" and "the pin is not what the repo says", and it
// costs one comparison.
func frozenPinMatches(scope *InstallScope, src *Source) error {
	if !installFrozen || src == nil {
		return nil
	}
	d, err := scopeDeclaration(scope)
	if err != nil || d == nil {
		return nil // frozenPrecheck already refused this
	}
	if !strings.EqualFold(strings.TrimSpace(d.SHA), src.SHA) {
		return frozenf("%s pins %s, but the install resolved %s. Nothing was installed",
			agentpakke.DeclarationPath, shortSHA(d.SHA), shortSHA(src.SHA))
	}
	return nil
}

// frozenComplete refuses an install that only partly landed.
//
// Without --frozen a skipped conflict and an unsupported kind are warnings,
// and the pin is then written as though everything went in: the pin says which
// revision was *attempted*, not what is on disk. That is tolerable for a
// developer who reads the warnings. It is exactly wrong for CI, which is why
// this is the one documented limitation --frozen closes rather than inherits.
func frozenComplete(result *installResult, scope *InstallScope) error {
	if !installFrozen || result == nil {
		return nil
	}
	if result.Conflicts > 0 {
		return frozenf("%d file(s) already exist and differ from %s, so the tree is not the pinned revision.\n"+
			"Pass %s to make it match, or resolve the differences",
			result.Conflicts, agentpakke.DeclarationPath, bold("--force"))
	}
	if len(result.Unsupported) > 0 {
		return frozenf("the agentpakke ships %s, which %s scope cannot hold, so the install is incomplete.\n"+
			"Narrow it with an %s list in %s, or install to a scope that supports them",
			strings.Join(result.Unsupported, ", "), scope.Name,
			bold("items"), agentpakke.DeclarationPath)
	}
	return nil
}
