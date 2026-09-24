package local

// Capabilities is what the benchmark has measured a model to be trusted with,
// per task class and per mode. It is generated in navikt/mlx-workspace by
// `mise run bench-capabilities` against the bar in
// reports/2026-09-24-local-vs-cloud-routing/design.md §2.
//
// It carries enums and nothing else. The sentences a verdict becomes are
// written in this binary, never in the served file: the dispatch policy is
// pasted into the system prompt of a cloud agent with full tool access, and
// this block may only move tasks between the local worker and the cloud, not
// say anything of its own.
//
// Forward compatible in both directions a new generator can move: a class id
// this binary does not know is ignored, and a verdict it does not know counts
// as not trusted. Neither refuses the manifest, because the safe reading of
// either is the one that keeps the task on the cloud.
type Capabilities struct {
	Classes map[string]ClassVerdict `json:"classes"`
}

// ClassVerdict is one class's verdict in each mode: "delegate" is a cloud
// orchestrator sending the task to the local worker, "local" a whole session
// on the local model.
type ClassVerdict struct {
	Delegate Verdict `json:"delegate"`
	Local    Verdict `json:"local"`
}

// Verdict is "trusted", "not-yet" or "cloud". Only [VerdictTrusted] means
// anything to the code that reads it; the other two are for people.
type Verdict string

const (
	VerdictTrusted Verdict = "trusted"
	VerdictNotYet  Verdict = "not-yet"
	VerdictCloud   Verdict = "cloud"
)

// TaskClasses is the allow-list of class ids, in the order they are named
// wherever a list of them is shown. Adding one is a nav-pilot change: it needs
// a sentence in the dispatch policy before a verdict about it can mean
// anything.
var TaskClasses = []string{"read-qa", "edit-single", "edit-multi-mechanical", "create-file", "debug"}

// DelegateTrusted reports, for each known class in [TaskClasses] order,
// whether it may be sent to the local worker. A nil receiver trusts nothing.
func (c *Capabilities) DelegateTrusted() (trusted, notTrusted []string) {
	for _, class := range TaskClasses {
		if c != nil && c.Classes[class].Delegate == VerdictTrusted {
			trusted = append(trusted, class)
		} else {
			notTrusted = append(notTrusted, class)
		}
	}
	return trusted, notTrusted
}
