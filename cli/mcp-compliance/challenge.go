package main

import "strings"

// challenge is one RFC 7235 auth-scheme with its auth-params (names lower-cased).
type challenge struct {
	scheme string
	params map[string]string
}

const tcharSet = "!#$%&'*+-.^_`|~0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// parseChallenges parses WWW-Authenticate values per RFC 7235 3.1.
// A token68 right after the scheme (`Negotiate YWJjZA==`) is skipped.
// shortcut: a malformed tail ends parsing; fine for Bearer.
func parseChallenges(values []string) []challenge {
	var out []challenge
	for _, v := range values {
		out = append(out, parseOne(v)...)
	}
	return out
}

func parseOne(s string) []challenge {
	var out []challenge
	i := 0
	skip := func() {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == ',') {
			i++
		}
	}
	space := func() {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
	}
	token := func() string {
		j := i
		for i < len(s) && strings.IndexByte(tcharSet, s[i]) >= 0 {
			i++
		}
		return s[j:i]
	}
	for {
		skip()
		scheme := token()
		if scheme == "" {
			return out
		}
		c := challenge{scheme: strings.ToLower(scheme), params: map[string]string{}}
		space()
		if j := token68End(s, i); j > i {
			i = j
			out = append(out, c)
			if i >= len(s) {
				return out
			}
			continue
		}
		for {
			skip()
			save := i
			name := token()
			if name == "" {
				break
			}
			space()
			if i >= len(s) || s[i] != '=' {
				i = save // next scheme
				break
			}
			i++
			space()
			var val string
			if i < len(s) && s[i] == '"' {
				i++
				var b strings.Builder
				for i < len(s) && s[i] != '"' {
					if s[i] == '\\' && i+1 < len(s) {
						i++
					}
					b.WriteByte(s[i])
					i++
				}
				if i >= len(s) {
					return append(out, c) // unterminated quote
				}
				i++
				val = b.String()
			} else {
				val = token()
			}
			if _, dup := c.params[strings.ToLower(name)]; !dup {
				c.params[strings.ToLower(name)] = val
			}
		}
		out = append(out, c)
		if i >= len(s) {
			return out
		}
	}
}

// bearerChallenges returns every Bearer challenge, scheme matched exactly and case-insensitively.
func bearerChallenges(values []string) (out []challenge) {
	for _, c := range parseChallenges(values) {
		if c.scheme == "bearer" {
			out = append(out, c)
		}
	}
	return out
}

// token68End returns the end of a token68 at i, or i when there is none. A lone
// trailing '=' is read as an auth-param with an empty value, not as padding.
func token68End(s string, i int) int {
	j := i
	for j < len(s) && (strings.IndexByte(tcharSet, s[j]) >= 0 || s[j] == '/') && s[j] != '\'' {
		j++
	}
	k := j
	for k < len(s) && s[k] == '=' {
		k++
	}
	if j == i || k-j == 1 {
		return i
	}
	m := k
	for m < len(s) && (s[m] == ' ' || s[m] == '\t') {
		m++
	}
	if m < len(s) && s[m] != ',' {
		return i
	}
	return k
}
