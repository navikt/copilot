package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type seenToken struct {
	sig string
	exp time.Time
}

const (
	maxSeenPerUser  = 20
	confirmTTL      = 5 * time.Minute
	confirmTokenKey = "confirmationToken"
)

var errConfirm = errors.New("confirmation token is invalid, expired or already used; call create_order again without confirmationToken to get a new preview")

// orderConfirm issues and verifies stateless HMAC-signed confirmation tokens
// bound to the user and the exact order parameters. Tokens are single use.
// shortcut: per-pod single-use set, move to Postgres if more replicas
type orderConfirm struct {
	mu   sync.Mutex
	keys []signingKey           // newest first
	seen map[string][]seenToken // userHash -> outstanding used tokens, oldest first
}

func newOrderConfirm(keys []signingKey) *orderConfirm {
	if len(keys) == 0 {
		panic("orderConfirm needs at least one key")
	}
	return &orderConfirm{keys: keys, seen: make(map[string][]seenToken)}
}

// paramsHash hashes the parsed order (environments sorted, token excluded) so
// argument order and whitespace do not matter. Call after parseCreateOrder validated fields.
func paramsHash(fields map[string]json.RawMessage) string {
	canon := map[string]any{}
	for k, v := range fields {
		switch k {
		case confirmTokenKey:
		case "environments":
			var envs []string
			_ = json.Unmarshal(v, &envs)
			sort.Strings(envs)
			canon[k] = envs
		default:
			var x any
			_ = json.Unmarshal(v, &x)
			canon[k] = x
		}
	}
	b, _ := json.Marshal(canon) // map keys are sorted by encoding/json
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// mac covers "<kid>.<exp>.<nonce>" so a token cannot be moved to another key.
func mac(key []byte, e, userHash, params string) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(e + "|" + userHash + "|" + params))
	return hex.EncodeToString(m.Sum(nil))
}

func (c *orderConfirm) issue(userHash, params string, now time.Time) (string, time.Time) {
	exp := now.Add(confirmTTL)
	var n [8]byte
	_, _ = rand.Read(n[:])
	// Random nonce: two previews of identical parameters get distinct single-use tokens.
	k := c.keys[0]
	e := k.kid + "." + strconv.FormatInt(exp.Unix(), 10) + "." + hex.EncodeToString(n[:])
	return e + "." + mac(k.key, e, userHash, params), exp
}

// consume verifies the token and marks it used. Any failure returns errConfirm.
func (c *orderConfirm) consume(token, userHash, params string, now time.Time) error {
	i := strings.LastIndex(token, ".")
	if i < 0 {
		return errConfirm
	}
	e, sig := token[:i], token[i+1:]
	kid, rest, _ := strings.Cut(e, ".")
	valid := 0
	for _, k := range c.keys {
		// Compare against every key so timing does not reveal which kid matched.
		ok := subtle.ConstantTimeCompare([]byte(sig), []byte(mac(k.key, e, userHash, params)))
		valid |= ok & subtle.ConstantTimeCompare([]byte(kid), []byte(k.kid))
	}
	if valid != 1 {
		return errConfirm
	}
	expStr, _, _ := strings.Cut(rest, ".")
	unix, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || !now.Before(time.Unix(unix, 0)) {
		return errConfirm
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	// Bounded per user so one user cannot fill the set and affect others.
	// Reject when full instead of evicting so uncharged confirms cannot
	// push a used token out of the set within its TTL.
	var kept []seenToken
	for _, st := range c.seen[userHash] {
		if !now.Before(st.exp) {
			continue
		}
		if st.sig == sig {
			return errConfirm
		}
		kept = append(kept, st)
	}
	if len(kept) >= maxSeenPerUser {
		return errConfirm
	}
	c.seen[userHash] = append(kept, seenToken{sig, time.Unix(unix, 0)})
	return nil
}

// remaining reports how many test persons the user may still order in the window.
func (q *userQuota) remaining(userHash string, now time.Time) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	used := 0
	for _, e := range q.entries[userHash] {
		if e.at.After(now.Add(-quotaWindow)) {
			used += e.antall
		}
	}
	return max(dailyPersonQuota-used, 0)
}
