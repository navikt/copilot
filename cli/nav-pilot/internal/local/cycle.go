package local

import "slices"

// MaxCyclePeriod is the longest cycle of tool calls the loop guard looks for.
//
// The same-call rules reset whenever the call changes, and a model that has
// been warned learns that fast: gpt-5-mini, told it had read the same file
// four times, went on reading it for as long as the run lasted by cycling the
// `view` tool between three ranges that all returned what it already had.
// Every call looked new; the three together were the same loop. Two or three
// steps cover the evasions seen so far; a longer cycle is closer to a plan
// than a tic.
const MaxCyclePeriod = 3

// RepeatedCycle finds a cycle of 2 to [MaxCyclePeriod] steps that steps, oldest
// first, end on, and how many times it repeats in full; a cycle seen once is no
// repeat and reports 0. A step is a call together with its normalised result,
// so two steps are equal only when the same call got the same answer: a cycle
// whose results change is not a repeat, and polling that makes progress never
// counts.
//
// A cycle whose steps are all equal is the one-call run the other rules
// already count, and is not reported. Where both periods fit, the one with
// more repeats wins.
func RepeatedCycle(steps []string) (period, reps int) {
	for p := 2; p <= MaxCyclePeriod && p <= len(steps); p++ {
		tail := steps[len(steps)-p:]
		if !slices.ContainsFunc(tail, func(s string) bool { return s != tail[0] }) {
			continue
		}
		start := len(steps) - p
		for start > 0 && steps[start-1] == steps[start-1+p] {
			start--
		}
		if r := (len(steps) - start) / p; r >= 2 && r > reps {
			period, reps = p, r
		}
	}
	return period, reps
}
