// Package canon computes the v1 contentHash: SHA-256 over RFC 8785 (JCS)
// canonical JSON.
package canon

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// Prefix names the canonicalisation behind a contentHash. A new field
// with a default, or any other change to canonical form, gets a new prefix;
// stored hashes are never recomputed.
const Prefix = "v1:sha256:"

// Hash is the v1 contentHash of v: SHA-256 over its RFC 8785 (JCS) form.
// The caller has normalised v (defaults filled in, sets sorted).
func Hash(v any) (string, error) {
	raw, err := jcs(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return Prefix + hex.EncodeToString(sum[:]), nil
}

// Hashes returns v's hash under every canonicalisation this build knows.
// Hashes compare only as whole strings, so a hash with another prefix never
// matches. Blocking by content checks every hash in this list, so blocked
// v1 content stays blocked once v2 exists: v2 code keeps computing v1 too.
func Hashes(v any) ([]string, error) {
	h, err := Hash(v)
	return []string{h}, err
}

// jcs serialises v per RFC 8785: object keys sorted by UTF-16 code units,
// ECMAScript number form, minimal string escapes.
func jcs(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := jcsValue(&buf, tree); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func jcsValue(buf *bytes.Buffer, v any) error {
	switch t := v.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		buf.WriteString(strconv.FormatBool(t))
	case json.Number:
		f, err := strconv.ParseFloat(string(t), 64)
		if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return fmt.Errorf("jcs: number %q out of range", t)
		}
		buf.WriteString(es6Number(f))
	case string:
		jcsString(buf, t)
	case []any:
		buf.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := jcsValue(buf, e); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		slices.SortFunc(keys, func(a, b string) int { return slices.Compare(utf16.Encode([]rune(a)), utf16.Encode([]rune(b))) })
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			jcsString(buf, k)
			buf.WriteByte(':')
			if err := jcsValue(buf, t[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	default:
		return fmt.Errorf("jcs: unexpected %T", v)
	}
	return nil
}

// es6Number formats f as ECMAScript Number.prototype.toString does.
func es6Number(f float64) string {
	if f == 0 {
		return "0" // also -0
	}
	format := byte('e')
	if a := math.Abs(f); a >= 1e-6 && a < 1e21 {
		format = 'f'
	}
	s := strconv.FormatFloat(f, format, -1, 64)
	if format == 'e' {
		// Go writes e-07 where ECMAScript writes e-7.
		if n := len(s); n >= 4 && s[n-4] == 'e' && s[n-2] == '0' {
			s = s[:n-2] + s[n-1:]
		}
	}
	return s
}

func jcsString(buf *bytes.Buffer, s string) {
	buf.WriteByte('"')
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\b':
			buf.WriteString(`\b`)
		case '\f':
			buf.WriteString(`\f`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(buf, `\u%04x`, r)
			} else {
				buf.WriteRune(r)
			}
		}
	}
	buf.WriteByte('"')
}
