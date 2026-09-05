package agentpakke

// The compatibility-range grammar, moved here from internal/provider's runtime
// gate (#504 U2). This package owns the manifest contract, and validation
// (checkSemantics) and the launch gate must accept and reject exactly the same
// strings — internal/provider imports this package and not the other way
// round, so one shared parser can only live here.

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Semver3 is a major.minor.patch triple. The contract's compatibility grammar
// has no ordering over prereleases and the reference refuses them outright, so
// three integers is the whole model.
type Semver3 [3]int

func (v Semver3) String() string { return fmt.Sprintf("%d.%d.%d", v[0], v[1], v[2]) }

func (v Semver3) Compare(o Semver3) int {
	for i := range v {
		switch {
		case v[i] < o[i]:
			return -1
		case v[i] > o[i]:
			return 1
		}
	}
	return 0
}

// Comparator is one clause of a compatibility range, e.g. ">=1.18.20".
type Comparator struct {
	Op string
	V  Semver3
}

// VersionRange is the conjunction of every comparator in a compatibility
// string: all must hold.
type VersionRange []Comparator

// comparatorOps are the operators the contract documents, longest first so
// ">=" is not read as ">" followed by a bad operand.
var comparatorOps = []string{">=", "<=", ">", "<", "="}

// ParseVersionRange parses the range grammar README.agentpakke.md documents:
// comma-separated comparators over semver, e.g. ">=1.18.20,<2". Operands may be
// partial ("2" means 2.0.0) — that is what makes an upper major bound writable.
func ParseVersionRange(s string) (VersionRange, error) {
	var rng VersionRange
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("empty comparator in %q", s)
		}
		op := ""
		for _, candidate := range comparatorOps {
			if strings.HasPrefix(part, candidate) {
				op = candidate
				break
			}
		}
		if op == "" {
			return nil, fmt.Errorf("comparator %q has no operator (expected one of >= > <= < =)", part)
		}
		v, err := parseVersionOperand(strings.TrimSpace(part[len(op):]))
		if err != nil {
			return nil, fmt.Errorf("comparator %q: %w", part, err)
		}
		rng = append(rng, Comparator{Op: op, V: v})
	}
	return rng, nil
}

var versionPartPattern = regexp.MustCompile(`^(0|[1-9]\d*)$`)

// parseVersionOperand parses one to three dot-separated numeric parts, filling
// the missing ones with zero.
func parseVersionOperand(s string) (Semver3, error) {
	var v Semver3
	if s == "" {
		return v, errors.New("missing version")
	}
	parts := strings.Split(s, ".")
	if len(parts) > 3 {
		return v, fmt.Errorf("version %q has more than three parts", s)
	}
	for i, part := range parts {
		if !versionPartPattern.MatchString(part) {
			return Semver3{}, fmt.Errorf("version %q has a non-numeric part %q", s, part)
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return Semver3{}, fmt.Errorf("version %q: %w", s, err)
		}
		v[i] = n
	}
	return v, nil
}

func (r VersionRange) Contains(v Semver3) bool {
	for _, c := range r {
		cmp := v.Compare(c.V)
		ok := false
		switch c.Op {
		case ">=":
			ok = cmp >= 0
		case ">":
			ok = cmp > 0
		case "<=":
			ok = cmp <= 0
		case "<":
			ok = cmp < 0
		case "=":
			ok = cmp == 0
		}
		if !ok {
			return false
		}
	}
	return true
}
