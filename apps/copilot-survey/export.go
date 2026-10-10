package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
)

// Redaction runs at export, not at write time: a false positive (a number
// that looks like a phone number) would otherwise destroy the answer for
// good, and the patterns can be improved without touching stored rows.
// Names are not caught: free text still needs a manual read before analysis.
var redactions = []struct {
	re  *regexp.Regexp
	tag string
}{
	{regexp.MustCompile(`(?i)\bhttps?://[^\s?#]*\?\S*`), "[url]"},
	{regexp.MustCompile(`[\w.+-]+@[\w-]+(?:\.[\w-]+)+`), "[e-post]"},
	{regexp.MustCompile(`\b\d{6} ?\d{5}\b`), "[fnr]"},
	{regexp.MustCompile(`(?:(?:\+|00)47 ?|\b)[2-9]\d(?: ?\d){6}\b`), "[telefon]"},
	{regexp.MustCompile(`\B@[A-Za-z0-9][A-Za-z0-9-]{0,38}`), "[brukernavn]"},
}

// ipCandidate finds what may be an IPv4 or IPv6 address; netip decides.
var ipCandidate = regexp.MustCompile(`(?:[0-9A-Fa-f]{0,4}:){2,7}(?:\d{1,3}(?:\.\d{1,3}){3}|[0-9A-Fa-f]{1,4})?|\b\d{1,3}(?:\.\d{1,3}){3}\b`)

func redact(s string) string {
	for _, r := range redactions[:2] {
		s = r.re.ReplaceAllString(s, r.tag)
	}
	// IP before the digit patterns, so 10.0.0.1 is not half a phone number.
	s = ipCandidate.ReplaceAllStringFunc(s, func(m string) string {
		if _, err := netip.ParseAddr(m); err == nil {
			return "[ip]"
		}
		return m
	})
	for _, r := range redactions[2:] {
		s = r.re.ReplaceAllString(s, r.tag)
	}
	return s
}

func redactValue(v any) any {
	switch t := v.(type) {
	case string:
		return redact(t)
	case []any:
		for i := range t {
			t[i] = redactValue(t[i])
		}
	case map[string]any:
		for k := range t {
			t[k] = redactValue(t[k])
		}
	}
	return v
}

// minSegment is the smallest group of respondents an export may show.
const minSegment = 5

const merged = "annet"

type exportRow struct {
	Answers          map[string]any    `json:"answers"`
	QuestionVersions map[string]int    `json:"question_versions"`
	Context          map[string]string `json:"context"`
}

// runExport reads one JSON object per line (answers, question_versions,
// context, as stored in survey_answers) and writes them back with free text
// redacted and every context value held by fewer than minSegment rows merged
// into «annet». A row whose combination of context values is still rarer
// than that gets «annet» for all of them.
func runExport(in io.Reader, out io.Writer) error {
	var rows []exportRow
	sc := bufio.NewScanner(in)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var raw struct {
			Answers          map[string]any `json:"answers"`
			QuestionVersions map[string]int `json:"question_versions"`
			Context          map[string]any `json:"context"`
		}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			return fmt.Errorf("row %d: %w", len(rows)+1, err)
		}
		ctx := map[string]string{}
		for k, v := range raw.Context {
			switch t := v.(type) {
			case string:
				ctx[k] = t
			case bool:
				ctx[k] = strconv.FormatBool(t)
			default:
				ctx[k] = fmt.Sprint(t)
			}
		}
		redactValue(raw.Answers)
		rows = append(rows, exportRow{raw.Answers, raw.QuestionVersions, ctx})
	}
	if err := sc.Err(); err != nil {
		return err
	}
	suppressSmallSegments(rows)
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	for _, r := range rows {
		if err := enc.Encode(r); err != nil {
			return err
		}
	}
	return nil
}

func suppressSmallSegments(rows []exportRow) {
	counts := map[string]map[string]int{}
	for _, r := range rows {
		for k, v := range r.Context {
			if counts[k] == nil {
				counts[k] = map[string]int{}
			}
			counts[k][v]++
		}
	}
	for _, r := range rows {
		for k, v := range r.Context {
			if counts[k][v] < minSegment {
				r.Context[k] = merged
			}
		}
	}
	key := func(c map[string]string) string { b, _ := json.Marshal(c); return string(b) }
	combos := map[string]int{}
	for _, r := range rows {
		combos[key(r.Context)]++
	}
	for _, r := range rows {
		if combos[key(r.Context)] < minSegment {
			for k := range r.Context {
				r.Context[k] = merged
			}
		}
	}
}
