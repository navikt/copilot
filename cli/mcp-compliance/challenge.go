package main

import "strings"

// challenge is one RFC 7235 auth-scheme with its auth-params (names lower-cased).
type challenge struct {
	scheme string
	params map[string]string
}

const tokenChars = "!#$%&'*+-.^_`|~0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// parseChallenges parses WWW-Authenticate values per RFC 7235 3.1.
// shortcut: token68 credentials (`Negotiate abc==`) are not recognised, and a
// malformed tail ends parsing; fine for Bearer, upgrade if another scheme matters.
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
		for i < len(s) && strings.IndexByte(tokenChars, s[i]) >= 0 {
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

// bearerChallenge returns the first Bearer challenge, scheme matched exactly and case-insensitively.
func bearerChallenge(values []string) (challenge, bool) {
	for _, c := range parseChallenges(values) {
		if c.scheme == "bearer" {
			return c, true
		}
	}
	return challenge{}, false
}
