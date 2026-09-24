package local

import (
	"regexp"
	"strings"
)

// The loop guard compares tool results to tell a stuck loop from progress. A
// byte-for-byte comparison let a loop through whenever its output embedded a
// clock: `gh run view` polled forever prints a new elapsed time on every call,
// and so does any command that stamps its log lines. NormaliseResult replaces
// the parts of a result that change on their own before the comparison, so
// two answers that differ only in that noise count as the same answer.
//
// The order matters. Timestamps and ids go first because they contain digit
// runs the last rule would otherwise split into several placeholders, and a
// duration goes before the bare number so "1.2s" and "900ms" normalise alike.
var resultNoise = []struct {
	re   *regexp.Regexp
	with string
}{
	{regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}(?::\d{2}(?:[.,]\d+)?)?(?:Z|[+-]\d{2}:?\d{2})?`), "<time>"},
	{regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`), "<id>"},
	{regexp.MustCompile(`\b(?:\d+(?:\.\d+)?(?:ns|µs|us|ms|h|m|s))+\b|\b\d+(?:\.\d+)?\s?(?:secs?|seconds?|mins?|minutes?|hours?)\b`), "<dur>"},
	{regexp.MustCompile(`\d+(?:[.,]\d+)*`), "<n>"},
}

// hexID is a git sha, a container id, a request id: seven or more hex digits.
// It needs a digit and a letter in it to count: without the digit, ordinary
// words spelled in a–f ("defaced", "acceded") would be replaced too, and a run
// of digits alone is a number, which the last rule handles.
var hexID = regexp.MustCompile(`(?i)\b(?:0x)?[0-9a-f]{7,}\b`)

// NormaliseResult returns s with timestamps, uuids, hex ids, durations and
// bare numbers replaced by placeholders, for comparing one tool result with
// another. It is only ever compared, never shown to anyone.
//
// ponytail: every digit run counts as noise, so a counter that is real
// progress ("12 of 40 done") looks unchanged too. The backstop rule still
// stops such a poll at the full threshold; the same-result rule trips on it at
// half. Narrow the last pattern if that turns out to stop real work.
func NormaliseResult(s string) string {
	s = resultNoise[0].re.ReplaceAllString(s, resultNoise[0].with)
	s = resultNoise[1].re.ReplaceAllString(s, resultNoise[1].with)
	s = hexID.ReplaceAllStringFunc(s, func(m string) string {
		h := strings.TrimPrefix(strings.ToLower(m), "0x")
		if strings.ContainsAny(h, "0123456789") && strings.ContainsAny(h, "abcdef") {
			return "<hex>"
		}
		return m
	})
	for _, r := range resultNoise[2:] {
		s = r.re.ReplaceAllString(s, r.with)
	}
	return s
}
