package main

import (
	"bytes"
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

const (
	confirmTTL      = 5 * time.Minute
	maxSeenTokens   = 10000
	confirmTokenKey = "confirmationToken"
)

var errConfirm = errors.New("confirmation token is invalid, expired or already used; call create_order again without confirmationToken to get a new preview")

// orderConfirm issues and verifies stateless HMAC-signed confirmation tokens
// bound to the user and the exact order parameters. Tokens are single use.
// shortcut: per-pod single-use set, move to Postgres if more replicas
type orderConfirm struct {
	mu   sync.Mutex
	key  [32]byte
	seen map[string]time.Time // token MAC -> expiry
}

func newOrderConfirm() *orderConfirm {
	c := &orderConfirm{seen: make(map[string]time.Time)}
	_, _ = rand.Read(c.key[:])
	return c
}

// paramsHash canonicalises the raw order arguments (sorted keys, compacted
// values, token excluded) so the same parameters always hash the same.
func paramsHash(fields map[string]json.RawMessage) string {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		if k != confirmTokenKey {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		var buf bytes.Buffer
		if json.Compact(&buf, fields[k]) != nil {
			buf.Write(fields[k])
		}
		h.Write([]byte(k + "=" + buf.String() + "\n"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (c *orderConfirm) mac(exp, userHash, params string) string {
	m := hmac.New(sha256.New, c.key[:])
	m.Write([]byte(exp + "|" + userHash + "|" + params))
	return hex.EncodeToString(m.Sum(nil))
}

func (c *orderConfirm) issue(userHash, params string, now time.Time) (string, time.Time) {
	exp := now.Add(confirmTTL)
	var n [8]byte
	_, _ = rand.Read(n[:])
	// Random nonce: two previews of identical parameters get distinct single-use tokens.
	e := strconv.FormatInt(exp.Unix(), 10) + "." + hex.EncodeToString(n[:])
	return e + "." + c.mac(e, userHash, params), exp
}

// consume verifies the token and marks it used. Any failure returns errConfirm.
func (c *orderConfirm) consume(token, userHash, params string, now time.Time) error {
	i := strings.LastIndex(token, ".")
	if i < 0 {
		return errConfirm
	}
	e, sig := token[:i], token[i+1:]
	if subtle.ConstantTimeCompare([]byte(sig), []byte(c.mac(e, userHash, params))) != 1 {
		return errConfirm
	}
	expStr, _, _ := strings.Cut(e, ".")
	unix, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || !now.Before(time.Unix(unix, 0)) {
		return errConfirm
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, used := c.seen[sig]; used {
		return errConfirm
	}
	if len(c.seen) >= maxSeenTokens {
		for k, v := range c.seen {
			if !now.Before(v) {
				delete(c.seen, k)
			}
		}
		if len(c.seen) >= maxSeenTokens {
			return errors.New("too many pending confirmations; try again in a few minutes")
		}
	}
	c.seen[sig] = time.Unix(unix, 0)
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
