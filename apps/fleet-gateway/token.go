package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	rtSealAAD = "fgw-rt-v1"
	// maxRefreshTokenLen is the expected upper bound for newly issued sealed
	// refresh tokens without groups: a ~2 KiB Entra RT plus metadata seals and
	// base64-encodes to ~3.1 KiB (3148 B measured), so 4 KiB leaves headroom
	// (~2x over a typical ~1 KiB Entra RT).
	maxRefreshTokenLen = 4 * 1024
	// maxLegacyRefreshTokenLen keeps the prior 16 KiB read-side cap so refresh
	// tokens issued before groups were dropped (up to ~200 group GUIDs) are not
	// rejected until they expire.
	maxLegacyRefreshTokenLen = 16 * 1024
	// refreshPendingTTL is how long an in-flight refresh blocks concurrent use of
	// the same RT; after a transient upstream failure it is also the retry delay.
	refreshPendingTTL = 60 * time.Second
)

type gatewaySession struct {
	ClientID     string    `json:"client_id"`
	Resource     string    `json:"resource"`
	EntraToken   string    `json:"entra_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"` //nolint:gosec // serialized only into SessionStore (sealed with RT_KEYS in Postgres)
	NAVIdent     string    `json:"nav_ident"`
	FID          string    `json:"fid,omitempty"`
	IssuedAt     time.Time `json:"issued_at"`
	Expiry       time.Time `json:"expiry"`
}

type issuedCode struct {
	ClientID    string         `json:"client_id"`
	RedirectURI string         `json:"redirect_uri"`
	Challenge   string         `json:"challenge"`
	Resource    string         `json:"resource"`
	Session     gatewaySession `json:"session"`
	EntraRT     string         `json:"entra_rt,omitempty"` //nolint:gosec // transient on 60s single-use code until /token seals it
	Created     time.Time      `json:"created"`
}

type rtPayload struct {
	V        int    `json:"v"`
	JTI      string `json:"jti"`
	FID      string `json:"fid"` // token family, constant across rotations
	IAT      int64  `json:"iat"`
	EXP      int64  `json:"exp"`           // idle deadline, moves forward at every rotation
	ABS      int64  `json:"abs,omitempty"` // hard cap for the family, fixed at login; 0 on legacy tokens
	ClientID string `json:"client_id"`
	Resource string `json:"resource"`
	NAVIdent string `json:"nav_ident"`
	EntraRT  string `json:"entra_rt"` //nolint:gosec // encrypted inside AES-GCM sealed client refresh token
}

func (g *Gateway) sealRefreshToken(p rtPayload) (string, error) {
	if g.rtKeys == nil {
		return "", errors.New("refresh token sealing keys not configured")
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	blob, err := g.rtKeys.Seal([]byte(rtSealAAD), raw)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(blob), nil
}

func (g *Gateway) openRefreshToken(token string) (*rtPayload, error) {
	if g.rtKeys == nil {
		return nil, errSealDecrypt
	}
	blob, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, errSealMalformed
	}
	plaintext, err := g.rtKeys.Open([]byte(rtSealAAD), blob)
	if err != nil {
		return nil, err
	}
	var p rtPayload
	if err := json.Unmarshal(plaintext, &p); err != nil {
		return nil, errSealMalformed
	}
	if p.V != 1 || p.JTI == "" || p.FID == "" || p.IAT <= 0 || p.EXP <= 0 || p.ClientID == "" || p.Resource == "" || p.NAVIdent == "" || p.EntraRT == "" {
		return nil, errSealMalformed
	}
	return &p, nil
}

// refreshGrace is the pair issued for a rotated RT. It is kept, sealed, for
// RefreshGraceWindow so a lost response or a second terminal can replay the old
// RT and get the same pair (OAuth 2.0 Security BCP 4.14.2).
type refreshGrace struct {
	AccessToken  string    `json:"access_token"`  //nolint:gosec // sealed in SessionStore for the grace window only
	RefreshToken string    `json:"refresh_token"` //nolint:gosec // sealed in SessionStore for the grace window only
	NewJTI       string    `json:"new_jti"`
	Expiry       time.Time `json:"expiry"`
}

// graceResponse returns the pair already issued for jti, while the grace window
// lasts and the new RT is still unused. Only the immediately previous RT
// qualifies: once the new RT has been rotated too, replaying the old one is reuse.
//
// The checks run after the pair is read, so a /revoke (which revokes the family)
// landing before this returns is seen. The old jti must be burned: the winner
// stores the pair before burning, and a pair whose burn failed must never be
// served, or the old RT and the new pair would be two live branches.
func (g *Gateway) graceResponse(ctx context.Context, jti, fid string) (map[string]any, bool, error) {
	raw, ok, err := g.store.Get(ctx, kindRefreshGrace, jti)
	if err != nil || !ok {
		return nil, false, err
	}
	var gr refreshGrace
	if err = json.Unmarshal(raw, &gr); err != nil {
		return nil, false, err
	}
	for _, k := range []string{"fam:" + fid, gr.NewJTI} {
		gone, rerr := g.store.IsRevoked(ctx, k)
		if rerr != nil || gone {
			return nil, false, rerr
		}
	}
	burned, err := g.store.IsRevoked(ctx, jti)
	if err != nil || !burned {
		return nil, false, err
	}
	ttl := gr.Expiry.Sub(g.now())
	if ttl <= 0 {
		return nil, false, nil
	}
	return map[string]any{
		"access_token":  gr.AccessToken,
		"token_type":    "Bearer",
		"expires_in":    int(ttl.Seconds()),
		"refresh_token": gr.RefreshToken,
	}, true, nil
}

// awaitGrace polls for the pair a concurrent refresh of the same RT is about to
// issue, for at most g.refreshWait.
func (g *Gateway) awaitGrace(ctx context.Context, jti, fid string) (map[string]any, bool, error) {
	deadline := time.Now().Add(g.refreshWait)
	for {
		resp, ok, err := g.graceResponse(ctx, jti, fid)
		if err != nil || ok || !time.Now().Before(deadline) {
			return resp, ok, err
		}
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func randomJTI() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (g *Gateway) tokenHandler(w http.ResponseWriter, r *http.Request) {
	bad := func(status int, errCode, desc string) {
		writeJSON(w, status, map[string]string{
			"error":             errCode,
			"error_description": desc,
		})
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxFormBodyBytes)
	if err := r.ParseForm(); err != nil {
		bad(http.StatusBadRequest, "invalid_request", "invalid form body")
		return
	}

	switch r.PostFormValue("grant_type") {
	case "authorization_code":
		g.handleAuthorizationCodeGrant(w, r, bad)
	case "refresh_token":
		if g.cfg.RTStorage != rtStorageClientEncrypted {
			bad(http.StatusBadRequest, "unsupported_grant_type", "only authorization_code is supported")
			return
		}
		g.handleRefreshTokenGrant(w, r, bad)
	default:
		if g.cfg.RTStorage == rtStorageClientEncrypted {
			bad(http.StatusBadRequest, "unsupported_grant_type", "supported grant_types are authorization_code and refresh_token")
			return
		}
		bad(http.StatusBadRequest, "unsupported_grant_type", "only authorization_code is supported")
	}
}

func (g *Gateway) handleAuthorizationCodeGrant(w http.ResponseWriter, r *http.Request, bad func(int, string, string)) {
	code := r.PostFormValue("code")
	if code == "" {
		bad(http.StatusBadRequest, "invalid_request", "code is required")
		return
	}

	// Look the client up before the code is consumed: a store error here answers
	// 503 Retry-After and the retry still finds the code. The client_id is
	// checked against the code below.
	known, mayRefresh, err := g.clientGrants(r.Context(), r.PostFormValue("client_id"))
	if err != nil {
		slog.Warn("load registered client failed", "error", err)
		w.Header().Set("Retry-After", "5")
		bad(http.StatusServiceUnavailable, "temporarily_unavailable", "session store unavailable")
		return
	}
	if !known {
		bad(http.StatusBadRequest, "invalid_grant", "invalid client")
		return
	}

	// Single-use: Pop deletes the code even if subsequent binding checks fail.
	issued, ok, err := g.popCode(r.Context(), code)
	if err != nil {
		slog.Warn("pop authorization code failed", "error", err)
		bad(http.StatusServiceUnavailable, "temporarily_unavailable", "session store unavailable")
		return
	}
	if !ok {
		bad(http.StatusBadRequest, "invalid_grant", "unknown, used, or expired code")
		return
	}

	if r.PostFormValue("client_id") != issued.ClientID {
		bad(http.StatusBadRequest, "invalid_grant", "client_id mismatch")
		return
	}
	if r.PostFormValue("redirect_uri") != issued.RedirectURI {
		bad(http.StatusBadRequest, "invalid_grant", "redirect_uri mismatch")
		return
	}
	if reqResource := r.PostFormValue("resource"); reqResource != "" && reqResource != issued.Resource {
		bad(http.StatusBadRequest, "invalid_target", "resource mismatch")
		return
	}

	verifier := r.PostFormValue("code_verifier")
	if !isValidCodeVerifier(verifier) ||
		subtle.ConstantTimeCompare([]byte(pkceS256(verifier)), []byte(issued.Challenge)) != 1 {
		bad(http.StatusBadRequest, "invalid_grant", "PKCE verification failed")
		return
	}

	userGone, err := g.userRevoked(r.Context(), issued.Session.NAVIdent, issued.Session.IssuedAt)
	if err != nil {
		slog.Warn("user lockout check failed", "error", err)
		bad(http.StatusServiceUnavailable, "temporarily_unavailable", "session store unavailable")
		return
	}
	if userGone {
		bad(http.StatusBadRequest, "invalid_grant", "session revoked")
		return
	}

	now := g.now()
	if issued.Session.IssuedAt.IsZero() {
		issued.Session.IssuedAt = now
	}
	// The code may come from a replica with other limits (rolling config change):
	// clamp to this replica's access-session cap and to the family cap.
	for _, maxExpiry := range []time.Time{now.Add(g.cfg.SessionMaxAge), issued.Session.IssuedAt.Add(g.cfg.SessionAbsoluteMaxAge)} {
		if issued.Session.Expiry.After(maxExpiry) {
			issued.Session.Expiry = maxExpiry
		}
	}
	ttl := issued.Session.Expiry.Sub(now)
	if ttl <= 0 {
		bad(http.StatusBadRequest, "invalid_grant", "session expired")
		return
	}

	accessToken, err := randomToken()
	if err != nil {
		bad(http.StatusInternalServerError, "server_error", "failed to issue token")
		return
	}

	refreshToken := ""
	if g.cfg.RTStorage == rtStorageClientEncrypted && issued.EntraRT != "" && mayRefresh {
		issued.Session.RefreshToken = ""
		jti, err := randomJTI()
		if err != nil {
			bad(http.StatusInternalServerError, "server_error", "failed to issue token")
			return
		}
		fid, err := randomJTI()
		if err != nil {
			bad(http.StatusInternalServerError, "server_error", "failed to issue token")
			return
		}
		refreshToken, err = g.sealRefreshToken(rtPayload{
			V:        1,
			JTI:      jti,
			FID:      fid,
			IAT:      issued.Session.IssuedAt.Unix(),
			ABS:      issued.Session.IssuedAt.Add(g.cfg.SessionAbsoluteMaxAge).Unix(),
			EXP:      g.rtExpiry(issued.Session.IssuedAt.Add(g.cfg.SessionAbsoluteMaxAge), now).Unix(),
			ClientID: issued.Session.ClientID,
			Resource: issued.Session.Resource,
			NAVIdent: issued.Session.NAVIdent,
			EntraRT:  issued.EntraRT,
		})
		if err != nil {
			bad(http.StatusInternalServerError, "server_error", "failed to issue token")
			return
		}
		issued.Session.FID = fid
	}

	if err := g.putSession(r.Context(), hashToken(accessToken), &issued.Session, ttl); err != nil {
		if errors.Is(err, errStoreFull) {
			markShed(w, "store_full")
			bad(http.StatusServiceUnavailable, "temporarily_unavailable", "session store capacity reached")
			return
		}
		slog.Warn("store session failed", "error", err)
		bad(http.StatusServiceUnavailable, "temporarily_unavailable", "session store unavailable")
		return
	}

	resp := map[string]any{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   int(ttl.Seconds()),
	}
	if refreshToken != "" {
		resp["refresh_token"] = refreshToken
	}
	g.renewClient(r.Context(), issued.Session.ClientID)
	writeJSON(w, http.StatusOK, resp)
}

// rtExpiry is when a refresh token issued at now stops working: the idle
// timeout from now, never later than the family hard cap.
func (g *Gateway) rtExpiry(hardCap, now time.Time) time.Time {
	return minTime(now.Add(g.cfg.SessionIdleTimeout), hardCap)
}

// retainUntil is how long a denylist entry for this token's family or jti must
// live: past the hard cap of every token the family can hold, whatever the cap
// is set to later. ABS is fixed at login and never below the token's own exp. A
// legacy token (no ABS, exp = iat + 12 h) has none, so retain to the larger of
// the current cap and the default 7 d, which only over-retains (shortcut: a cap
// above 7 d lowered before a legacy token is replayed is not covered; legacy
// tokens live 12 h after the rollout).
func (g *Gateway) retainUntil(p *rtPayload) time.Time {
	if p.ABS > 0 {
		return time.Unix(p.ABS, 0).UTC()
	}
	return time.Unix(p.IAT, 0).UTC().Add(max(g.cfg.SessionAbsoluteMaxAge, defaultSessionAbsoluteMaxAge))
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

func (g *Gateway) handleRefreshTokenGrant(w http.ResponseWriter, r *http.Request, bad func(int, string, string)) {
	outcome := "ok"
	navIdent := ""
	defer func() {
		slog.Info("audit refresh", "outcome", outcome, "navident", sanitizeLogValue(navIdent))
	}()

	rawRT := r.PostFormValue("refresh_token")
	if rawRT == "" || len(rawRT) > maxLegacyRefreshTokenLen {
		outcome = "invalid_request"
		bad(http.StatusBadRequest, "invalid_grant", "invalid refresh token")
		return
	}

	clientID := r.PostFormValue("client_id")
	known, mayRefresh, err := g.clientGrants(r.Context(), clientID)
	if err != nil {
		outcome = "store_error"
		slog.Warn("load registered client failed", "error", err)
		w.Header().Set("Retry-After", "5")
		bad(http.StatusServiceUnavailable, "temporarily_unavailable", "session store unavailable")
		return
	}
	if !known {
		outcome = "client_not_allowed"
		bad(http.StatusBadRequest, "invalid_grant", "invalid client")
		return
	}
	if !mayRefresh {
		outcome = "grant_not_registered"
		bad(http.StatusBadRequest, "unauthorized_client", "refresh_token grant not registered for client")
		return
	}

	payload, err := g.openRefreshToken(rawRT)
	if err != nil {
		outcome = "decrypt_failed"
		bad(http.StatusBadRequest, "invalid_grant", "invalid refresh token")
		return
	}
	navIdent = payload.NAVIdent

	if clientID != payload.ClientID {
		outcome = "client_mismatch"
		bad(http.StatusBadRequest, "invalid_grant", "client_id mismatch")
		return
	}
	if reqResource := r.PostFormValue("resource"); reqResource != "" && reqResource != payload.Resource {
		outcome = "resource_mismatch"
		bad(http.StatusBadRequest, "invalid_grant", "resource mismatch")
		return
	}

	// A resource that no longer matches a route (retired or moved) would mint a token no route accepts: force re-login.
	if !g.cfg.AllowedResources[payload.Resource] {
		outcome = "resource_retired"
		bad(http.StatusBadRequest, "invalid_grant", "resource no longer available; log in again")
		return
	}

	issuedAt := time.Unix(payload.IAT, 0).UTC()
	// payload.IAT is the original login and never changes across rotations, so
	// refreshing cannot move the hard cap. A legacy RT carries the same IAT, so
	// it is treated as login time. payload.EXP is the idle deadline: each
	// rotation moves it forward (rtExpiry), up to the hard cap.
	rtExpiry := time.Unix(payload.EXP, 0).UTC()
	// famDeadline is when the last token of this family can die, whatever the cap
	// is set to later: ABS is fixed at login, so lowering then raising
	// SESSION_ABSOLUTE_MAX_AGE cannot shorten denylist retention below any issued
	// token. A legacy token has no ABS; its family gets one at this rotation.
	famDeadline := issuedAt.Add(g.cfg.SessionAbsoluteMaxAge)
	absExpiry := famDeadline // enforced cap: also follows a lowered setting
	if payload.ABS > 0 {
		famDeadline = time.Unix(payload.ABS, 0).UTC()
		absExpiry = minTime(famDeadline, absExpiry)
	}
	retain := g.retainUntil(payload)
	if !g.now().Before(absExpiry) {
		outcome = "session_max_age_expired"
		bad(http.StatusBadRequest, "invalid_grant", "session expired")
		return
	}

	unavailable := func(err error) {
		outcome = "store_error"
		slog.Warn("refresh token denylist failed", "error", err)
		bad(http.StatusServiceUnavailable, "temporarily_unavailable", "session store unavailable")
	}
	userGone, err := g.userRevoked(r.Context(), payload.NAVIdent, issuedAt)
	if err != nil {
		unavailable(err)
		return
	}
	if userGone {
		outcome = "user_revoked"
		bad(http.StatusBadRequest, "invalid_grant", "session revoked")
		return
	}
	famKey := "fam:" + payload.FID
	revoked, err := g.store.IsRevoked(r.Context(), famKey)
	if err != nil {
		unavailable(err)
		return
	}
	if revoked {
		outcome = "family_revoked"
		bad(http.StatusBadRequest, "invalid_grant", "refresh token already used or revoked")
		return
	}
	used, err := g.store.IsRevoked(r.Context(), payload.JTI)
	if err != nil {
		unavailable(err)
		return
	}
	if used {
		// Inside the grace window the immediately previous RT gets the same pair again.
		resp, ok, gerr := g.graceResponse(r.Context(), payload.JTI, payload.FID)
		if gerr != nil {
			unavailable(gerr)
			return
		}
		if ok {
			outcome = "grace_replay"
			writeJSON(w, http.StatusOK, resp)
			return
		}
		// Replay: someone holds an already-rotated RT. Kill the family (until the
		// hard cap: later rotations live past this token's own EXP) so a thief
		// who refreshed first loses the newest RT too.
		if err = g.store.Revoke(r.Context(), famKey, retain); err != nil {
			unavailable(err)
			return
		}
		outcome = "replayed_family_revoked"
		bad(http.StatusBadRequest, "invalid_grant", "refresh token already used or revoked")
		return
	}
	// After replay and grace handling: a replay past the idle deadline still
	// kills the family, and a retry inside the grace window still works.
	if !g.now().Before(rtExpiry) {
		outcome = "session_idle_expired"
		bad(http.StatusBadRequest, "invalid_grant", "session expired")
		return
	}

	// Gate against concurrent double use with a short-lived pending claim. The jti
	// is only denylisted for good after Entra accepts or permanently rejects the RT,
	// so a transient Entra failure does not burn the user's RT (retry after the
	// pending TTL). Trade-off: a crash between Entra success and Revoke leaves the
	// old RT usable once more after the TTL.
	if r.Context().Err() != nil {
		outcome = "client_canceled"
		return
	}
	// Detach from client cancellation before claiming pend:<jti> so a client abort
	// during the claim or Entra call cannot leave pend:<jti> held for
	// refreshPendingTTL with the jti unburned and no grace pair.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), defaultOutboundTimeout)
	defer cancel()

	pendExpiry := g.now().Add(refreshPendingTTL)
	if pendExpiry.After(absExpiry) {
		pendExpiry = absExpiry
	}
	won, err := g.store.RevokeOnce(ctx, "pend:"+payload.JTI, pendExpiry)
	if err != nil {
		unavailable(err)
		return
	}
	if !won {
		// A concurrent refresh of the same RT: wait briefly for its pair.
		resp, ok, gerr := g.awaitGrace(r.Context(), payload.JTI, payload.FID)
		if gerr != nil {
			unavailable(gerr)
			return
		}
		if ok {
			outcome = "grace_concurrent"
			writeJSON(w, http.StatusOK, resp)
			return
		}
		// A legit client may be retrying after a transient upstream error; do not
		// make it drop its RT. Retry-After stays inside the grace window so a retry
		// after a slow but successful refresh still gets the pair.
		outcome = "refresh_in_flight"
		retry := min(refreshPendingTTL, absExpiry.Sub(g.now()), g.cfg.RefreshGraceWindow/2)
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(retry.Seconds()))))
		markShed(w, "refresh_in_flight")
		bad(http.StatusServiceUnavailable, "temporarily_unavailable", "refresh in progress, retry later")
		return
	}

	out, status, body, err := g.redeemEntraRefreshToken(ctx, payload.EntraRT)
	if err != nil || status >= 500 || status == http.StatusTooManyRequests {
		outcome = "upstream_unavailable"
		slog.Warn("upstream refresh unavailable", "status", status)
		w.Header().Set("Retry-After", strconv.Itoa(int(refreshPendingTTL.Seconds())))
		bad(http.StatusServiceUnavailable, "temporarily_unavailable", "upstream unavailable, retry later")
		return
	}
	if status == http.StatusOK {
		// /revoke or a family revoke may have landed while Entra was in flight.
		for _, k := range []string{famKey, payload.JTI} {
			gone, rerr := g.store.IsRevoked(ctx, k)
			if rerr != nil {
				unavailable(rerr)
				return
			}
			if gone {
				outcome = "revoked_in_flight"
				bad(http.StatusBadRequest, "invalid_grant", "refresh token already used or revoked")
				return
			}
		}
	}
	if status != http.StatusOK {
		if err = g.store.Revoke(ctx, payload.JTI, retain); err != nil {
			unavailable(err)
			return
		}
		outcome = "upstream_error"
		errCode, aadsts := safeUpstreamErrorDetails(body)
		slog.Warn("upstream refresh failed", "status", status, "error_code", errCode, "aadsts", aadsts)
		bad(http.StatusBadRequest, "invalid_grant", "upstream refresh failed")
		return
	}
	// From here the old jti is burned only after the grace pair is stored, so a
	// replay never sees it used without a pair. A failure before that leaves the
	// RT blocked by the pending claim only, as after a transient upstream error.
	newAT, _ := out["access_token"].(string)
	if newAT == "" {
		outcome = "upstream_error"
		slog.Warn("upstream refresh response missing access_token")
		bad(http.StatusBadRequest, "invalid_grant", "upstream refresh failed")
		return
	}

	now := g.now()
	if !now.Before(absExpiry) {
		outcome = "session_max_age_expired"
		bad(http.StatusBadRequest, "invalid_grant", "session expired")
		return
	}
	sessionTTL := computeSessionTTL(out["expires_in"], g.cfg.SessionMaxAge)
	sessExpiry := now.Add(sessionTTL)
	if sessExpiry.After(absExpiry) {
		sessExpiry = absExpiry
	}
	ttl := sessExpiry.Sub(now)
	if ttl <= 0 {
		outcome = "session_max_age_expired"
		bad(http.StatusBadRequest, "invalid_grant", "session expired")
		return
	}

	accessToken, err := randomToken()
	if err != nil {
		outcome = "server_error"
		bad(http.StatusInternalServerError, "server_error", "failed to issue token")
		return
	}
	newJTI, err := randomJTI()
	if err != nil {
		outcome = "server_error"
		bad(http.StatusInternalServerError, "server_error", "failed to issue token")
		return
	}
	newEntraRT, _ := out["refresh_token"].(string)
	if newEntraRT == "" {
		newEntraRT = payload.EntraRT
	}
	newRT, err := g.sealRefreshToken(rtPayload{
		V:        1,
		JTI:      newJTI,
		FID:      payload.FID,
		IAT:      payload.IAT,
		ABS:      famDeadline.Unix(),
		EXP:      g.rtExpiry(absExpiry, now).Unix(),
		ClientID: payload.ClientID,
		Resource: payload.Resource,
		NAVIdent: payload.NAVIdent,
		EntraRT:  newEntraRT,
	})
	if err != nil {
		outcome = "server_error"
		bad(http.StatusInternalServerError, "server_error", "failed to issue token")
		return
	}

	newSess := &gatewaySession{
		ClientID:     payload.ClientID,
		Resource:     payload.Resource,
		EntraToken:   newAT,
		RefreshToken: "",
		NAVIdent:     payload.NAVIdent,
		FID:          payload.FID,
		IssuedAt:     issuedAt,
		Expiry:       sessExpiry,
	}
	if err = g.putSession(ctx, hashToken(accessToken), newSess, ttl); err != nil {
		outcome = "store_error"
		if errors.Is(err, errStoreFull) {
			markShed(w, "store_full")
			bad(http.StatusServiceUnavailable, "temporarily_unavailable", "session store capacity reached")
			return
		}
		slog.Warn("store refreshed session failed", "error", err)
		bad(http.StatusServiceUnavailable, "temporarily_unavailable", "session store unavailable")
		return
	}
	// Store the pair, then burn the old jti. graceResponse serves the pair only
	// once the jti is burned, so a failed or crashed burn leaves the pair unused
	// and concurrent users get 503 Retry-After, never a second live branch.
	// A lost grace pair only costs the grace window, so a write failure is not fatal.
	graceTTL := min(g.cfg.RefreshGraceWindow, absExpiry.Sub(now))
	if raw, merr := json.Marshal(refreshGrace{AccessToken: accessToken, RefreshToken: newRT, NewJTI: newJTI, Expiry: sessExpiry}); merr != nil {
		slog.Warn("marshal refresh grace failed", "error", merr)
	} else if perr := g.store.Put(ctx, kindRefreshGrace, payload.JTI, raw, graceTTL); perr != nil {
		slog.Warn("store refresh grace failed", "error", perr)
	}
	if err = g.store.Revoke(ctx, payload.JTI, retain); err != nil {
		unavailable(err)
		return
	}

	g.renewClient(ctx, clientID)
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    int(ttl.Seconds()),
		"refresh_token": newRT,
	})
}

func (g *Gateway) redeemEntraRefreshToken(ctx context.Context, entraRT string) (map[string]any, int, []byte, error) {
	form := url.Values{
		"client_id":     {g.cfg.AzureClientID},
		"client_secret": {g.cfg.AzureClientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {entraRT},
		"scope":         {g.cfg.EntraScope},
	}
	body, status, err := g.postEntraToken(ctx, form)
	if err != nil {
		return nil, status, body, err
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, status, body, err
	}
	return out, status, body, nil
}

func (g *Gateway) revokeHandler(w http.ResponseWriter, r *http.Request) {
	bad := func(status int, errCode, desc string) {
		writeJSON(w, status, map[string]string{
			"error":             errCode,
			"error_description": desc,
		})
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxFormBodyBytes)
	if err := r.ParseForm(); err != nil {
		bad(http.StatusBadRequest, "invalid_request", "invalid form body")
		return
	}

	clientID := r.PostFormValue("client_id")
	if clientID == "" {
		bad(http.StatusBadRequest, "invalid_request", "client_id is required")
		return
	}

	token := r.PostFormValue("token")
	if token != "" && len(token) <= maxLegacyRefreshTokenLen {
		tokenHash := hashToken(token)
		sess, ok, err := g.getSession(r.Context(), tokenHash)
		if err != nil {
			slog.Warn("revoke session lookup failed", "error", err)
			bad(http.StatusServiceUnavailable, "temporarily_unavailable", "session store unavailable")
			return
		}
		if ok {
			if sess.ClientID != clientID {
				slog.Warn("revoke client mismatch")
				writeJSON(w, http.StatusOK, map[string]any{})
				return
			}
			// Consent first: if its delete fails the session stays, so a retry still finds it.
			if err = g.forgetConsent(r.Context(), sess.NAVIdent, sess.ClientID, sess.Resource); err != nil {
				slog.Warn("revoke consent delete failed", "error", err)
				bad(http.StatusServiceUnavailable, "temporarily_unavailable", "session store unavailable")
				return
			}
			if err = g.store.Delete(r.Context(), kindSession, tokenHash); err != nil {
				slog.Warn("revoke session delete failed", "error", err)
				bad(http.StatusServiceUnavailable, "temporarily_unavailable", "session store unavailable")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{})
			return
		}
		payload, err := g.openRefreshToken(token)
		if err != nil {
			if errors.Is(err, errSealUnknownKID) && len(token) != 43 {
				slog.Warn("revoke refresh token unknown key id", "error", err)
				w.Header().Set("Retry-After", "5")
				bad(http.StatusServiceUnavailable, "temporarily_unavailable", "refresh token key unavailable")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{})
			return
		}
		if payload.ClientID != clientID {
			slog.Warn("revoke client mismatch")
			writeJSON(w, http.StatusOK, map[string]any{})
			return
		}
		if err := g.forgetConsent(r.Context(), payload.NAVIdent, payload.ClientID, payload.Resource); err != nil {
			slog.Warn("revoke consent delete failed", "error", err)
			bad(http.StatusServiceUnavailable, "temporarily_unavailable", "session store unavailable")
			return
		}
		expiresAt := g.retainUntil(payload)
		if _, err := g.store.RevokeOnce(r.Context(), payload.JTI, expiresAt); err != nil {
			slog.Warn("revoke refresh token failed", "error", err)
			bad(http.StatusServiceUnavailable, "temporarily_unavailable", "session store unavailable")
			return
		}
		// Revoke the family too (RFC 7009 2.1), so a grace pair already read
		// by a concurrent replay fails its family recheck.
		if err := g.store.Revoke(r.Context(), "fam:"+payload.FID, g.retainUntil(payload)); err != nil {
			slog.Warn("revoke refresh family failed", "error", err)
			bad(http.StatusServiceUnavailable, "temporarily_unavailable", "session store unavailable")
			return
		}
		if err := g.store.Delete(r.Context(), kindRefreshGrace, payload.JTI); err != nil {
			slog.Warn("revoke refresh grace failed", "error", err)
			bad(http.StatusServiceUnavailable, "temporarily_unavailable", "session store unavailable")
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func pkceS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func isValidCodeVerifier(verifier string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	for i := 0; i < len(verifier); i++ {
		c := verifier[i]
		isAlpha := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		isDigit := c >= '0' && c <= '9'
		isUnreserved := c == '-' || c == '.' || c == '_' || c == '~'
		if !isAlpha && !isDigit && !isUnreserved {
			return false
		}
	}
	return true
}
