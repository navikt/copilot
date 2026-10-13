package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	cases := map[string]string{
		"skriv til ola.nordmann@example.com":       "skriv til [e-post]",
		"ring 912 34 567 eller +47 22334455":       "ring [telefon] eller [telefon]",
		"fnr 01019912345 og 010199 12345":          "fnr [fnr] og [fnr]",
		"fra 10.0.0.1 og 2001:db8::1":              "fra [ip] og [ip]",
		"+4791234567 og 004791234567":              "[telefon] og [telefon]",
		"::1, fe80:: og ::ffff:192.0.2.1":          "[ip], [ip] og [ip]",
		"spør @octocat om det":                     "spør [brukernavn] om det",
		"se https://example.com/a?q=abc og videre": "se [url] og videre",
		// Negative cases: left as they are.
		"https://example.com/docs uten spørring": "https://example.com/docs uten spørring",
		"versjon 2026.09 og 1.2.3":               "versjon 2026.09 og 1.2.3",
		"klokka 10:30:00, 300 ganger, 1234567":   "klokka 10:30:00, 300 ganger, 1234567",
		"999.1.1.1 er ingen adresse":             "999.1.1.1 er ingen adresse",
		"Kari på team X":                         "Kari på team X",
	}
	for in, want := range cases {
		if got := redact(in); got != want {
			t.Errorf("redact(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExportRedactsAndSuppressesSmallSegments(t *testing.T) {
	var in strings.Builder
	row := func(client, os string, text any) {
		b, _ := json.Marshal(map[string]any{
			"answers":           map[string]any{"q1": "a", "q2": text},
			"question_versions": map[string]int{"q1": 1},
			"context":           map[string]any{"client": client, "os": os, "version": "2026.09", "local_models": false},
		})
		in.Write(append(b, '\n'))
	}
	for i := range 6 {
		row("copilot", "darwin", fmt.Sprintf("svar %d", i))
	}
	for range 5 {
		row("opencode", "linux", []any{"x", "mail a@b.no"})
	}
	row("pi", "windows", "eneste")
	row("copilot", "linux", "sjelden kombinasjon")

	var out bytes.Buffer
	if err := runExport(strings.NewReader(in.String()), &out); err != nil {
		t.Fatal(err)
	}
	var rows []exportRow
	for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var r exportRow
		if err := json.Unmarshal([]byte(l), &r); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, r)
	}
	if len(rows) != 13 {
		t.Fatalf("got %d rows", len(rows))
	}
	if got := rows[6].Answers["q2"].([]any)[1]; got != "mail [e-post]" {
		t.Errorf("free text in a list not redacted: %v", got)
	}
	if c := rows[0].Context; c["client"] != "copilot" || c["os"] != "darwin" || c["local_models"] != "false" {
		t.Errorf("segment of 6 changed: %v", c)
	}
	// pi (1 row) and windows (1 row) are merged; copilot+linux is a
	// combination of 1 and loses all its values.
	if c := rows[11].Context; c["client"] != merged || c["os"] != merged {
		t.Errorf("segment of 1 kept: %v", c)
	}
	if c := rows[12].Context; c["client"] != merged || c["os"] != merged || c["version"] != merged {
		t.Errorf("rare combination kept: %v", c)
	}
}
