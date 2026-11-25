// Package store holds OAuth sessions, codes and tokens in memory.
package store

import (
	"errors"
	"sync"
	"time"
)

// ErrNotFound means the entry is missing or expired.
var ErrNotFound = errors.New("not found")

// TODO: If we need to scale beyond a single replica, replace in-memory maps with Redis
// to share sessions and tokens across pods. Client registrations are not here:
// the client_id carries its own signed registration (see clientid.go).

// AuthSession is an authorization request waiting for the GitHub callback.
type AuthSession struct {
	ClientID            string
	ClientState         string
	RedirectURI         string
	CodeChallenge       string
	CodeChallengeMethod string
	CreatedAt           time.Time
}

// AuthCode is an issued authorization code waiting for the token request.
type AuthCode struct {
	ClientID           string
	GitHubAccessToken  string
	GitHubRefreshToken string
	GitHubExpiresAt    time.Time
	CodeChallenge      string
	RedirectURI        string
	UserLogin          string
	UserID             int64
	CreatedAt          time.Time
}

// TokenData is what an access token grants.
type TokenData struct {
	GitHubAccessToken  string
	GitHubRefreshToken string
	GitHubExpiresAt    time.Time
	UserLogin          string
	UserID             int64
	ExpiresAt          time.Time
	// ClientID and RefreshToken tie the access token to its client and to the
	// refresh token issued with it, so a refresh or a revocation can remove both.
	// Both are empty on tokens issued before #2746.
	ClientID     string
	RefreshToken string
}

// RefreshTokenData is what a refresh token grants.
type RefreshTokenData struct {
	// ClientID is the client the token was issued to. The refresh grant
	// requires the caller to present it, so a stolen refresh token is not
	// usable on its own (GHSA-7hwf-488h-59x8).
	ClientID           string
	GitHubRefreshToken string
	UserLogin          string
	UserID             int64
	CreatedAt          time.Time
	// AccessToken is the access token issued together with this refresh token.
	AccessToken string
}

// TokenStore is safe for concurrent use.
type TokenStore struct {
	authSessions  map[string]*AuthSession
	authCodes     map[string]*AuthCode
	tokens        map[string]*TokenData
	refreshTokens map[string]*RefreshTokenData
	mu            sync.RWMutex
}

// NewTokenStore starts a goroutine that purges expired entries.
func NewTokenStore() *TokenStore {
	store := &TokenStore{
		authSessions:  make(map[string]*AuthSession),
		authCodes:     make(map[string]*AuthCode),
		tokens:        make(map[string]*TokenData),
		refreshTokens: make(map[string]*RefreshTokenData),
	}

	go store.cleanupExpired()

	return store
}

const maxStoreEntries = 10000

// SaveAuthSession evicts an entry when the store is full.
func (s *TokenStore) SaveAuthSession(state string, session *AuthSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.authSessions) >= maxStoreEntries {
		now := time.Now()
		for k, v := range s.authSessions {
			if now.Sub(v.CreatedAt) > 10*time.Minute {
				delete(s.authSessions, k)
			}
		}
		if len(s.authSessions) >= maxStoreEntries {
			for k := range s.authSessions {
				delete(s.authSessions, k)
				break
			}
		}
	}
	s.authSessions[state] = session
}

// GetAuthSession returns ErrNotFound after 10 minutes.
func (s *TokenStore) GetAuthSession(state string) (*AuthSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.authSessions[state]
	if !ok {
		return nil, ErrNotFound
	}
	if time.Since(session.CreatedAt) > 10*time.Minute {
		return nil, ErrNotFound
	}
	return session, nil
}

// DeleteAuthSession removes the session for state.
func (s *TokenStore) DeleteAuthSession(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.authSessions, state)
}

// SaveAuthCode evicts an entry when the store is full.
func (s *TokenStore) SaveAuthCode(code string, authCode *AuthCode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.authCodes) >= maxStoreEntries {
		now := time.Now()
		for k, v := range s.authCodes {
			if now.Sub(v.CreatedAt) > 10*time.Minute {
				delete(s.authCodes, k)
			}
		}
		if len(s.authCodes) >= maxStoreEntries {
			for k := range s.authCodes {
				delete(s.authCodes, k)
				break
			}
		}
	}
	s.authCodes[code] = authCode
}

// GetAuthCode returns the code or ErrNotFound.
func (s *TokenStore) GetAuthCode(code string) (*AuthCode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	authCode, ok := s.authCodes[code]
	if !ok {
		return nil, ErrNotFound
	}
	return authCode, nil
}

// DeleteAuthCode removes the code.
func (s *TokenStore) DeleteAuthCode(code string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.authCodes, code)
}

// SaveToken evicts an entry when the store is full.
func (s *TokenStore) SaveToken(token string, data *TokenData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveTokenLocked(token, data)
}

func (s *TokenStore) saveTokenLocked(token string, data *TokenData) {
	if len(s.tokens) >= maxStoreEntries {
		now := time.Now()
		for k, v := range s.tokens {
			if now.After(v.ExpiresAt) {
				delete(s.tokens, k)
			}
		}
		if len(s.tokens) >= maxStoreEntries {
			for k := range s.tokens {
				delete(s.tokens, k)
				break
			}
		}
	}
	s.tokens[token] = data
}

// GetToken returns ErrNotFound for expired tokens.
func (s *TokenStore) GetToken(token string) (*TokenData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.tokens[token]
	if !ok {
		return nil, ErrNotFound
	}
	if time.Now().After(data.ExpiresAt) {
		return nil, ErrNotFound
	}
	return data, nil
}

// SaveRefreshToken evicts an entry when the store is full.
func (s *TokenStore) SaveRefreshToken(token string, data *RefreshTokenData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveRefreshTokenLocked(token, data)
}

func (s *TokenStore) saveRefreshTokenLocked(token string, data *RefreshTokenData) {
	if len(s.refreshTokens) >= maxStoreEntries {
		now := time.Now()
		for k, v := range s.refreshTokens {
			if now.Sub(v.CreatedAt) > 30*24*time.Hour {
				delete(s.refreshTokens, k)
			}
		}
		if len(s.refreshTokens) >= maxStoreEntries {
			for k := range s.refreshTokens {
				delete(s.refreshTokens, k)
				break
			}
		}
	}
	s.refreshTokens[token] = data
}

// GetRefreshToken returns ErrNotFound after 30 days.
func (s *TokenStore) GetRefreshToken(token string) (*RefreshTokenData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.refreshTokens[token]
	if !ok {
		return nil, ErrNotFound
	}
	if time.Since(data.CreatedAt) > 30*24*time.Hour {
		return nil, ErrNotFound
	}
	return data, nil
}

// DeleteRefreshToken removes the refresh token.
func (s *TokenStore) DeleteRefreshToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.refreshTokens, token)
}

// DeleteToken removes an access token without touching its refresh token, as
// cleanupExpired does.
func (s *TokenStore) DeleteToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, token)
}

// Sizes returns the number of stored entries of each kind.
func (s *TokenStore) Sizes() (authSessions, tokens, refreshTokens int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.authSessions), len(s.tokens), len(s.refreshTokens)
}

// RotateRefreshToken consumes oldRefresh and saves the new pair under one
// lock. It returns ErrNotFound, and saves nothing, if oldRefresh is gone or
// bound to another client: a revocation or a concurrent refresh won the race
// while the caller was talking to GitHub. The access token issued with
// oldRefresh is retired too (#2746).
func (s *TokenStore) RotateRefreshToken(oldRefresh, clientID, access string, at *TokenData, refresh string, rt *RefreshTokenData) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.refreshTokens[oldRefresh]
	if !ok || old.ClientID == "" || old.ClientID != clientID {
		return ErrNotFound
	}
	delete(s.refreshTokens, oldRefresh)
	delete(s.tokens, old.AccessToken)
	s.saveTokenLocked(access, at)
	s.saveRefreshTokenLocked(refresh, rt)
	return nil
}

// RevokeResult is the outcome of RevokeToken.
type RevokeResult int

// Outcomes of RevokeToken.
const (
	RevokeUnknown  RevokeResult = iota // no such token: RFC 7009 still answers 200
	RevokeDone                         // token and its pair removed
	RevokeNotOwner                     // token belongs to another client; nothing removed
)

// RevokeToken removes an access or refresh token, and the token issued with it,
// if clientID is the client it was issued to. Lookup and delete happen under one
// lock; a refresh in flight then fails in RotateRefreshToken instead of saving
// a new pair for a revoked grant.
func (s *TokenStore) RevokeToken(token, clientID string) RevokeResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	if at, ok := s.tokens[token]; ok {
		// An access token from before #2746 has no ClientID, so no client can
		// prove ownership; it expires within the hour.
		if at.ClientID == "" || at.ClientID != clientID {
			return RevokeNotOwner
		}
		delete(s.tokens, token)
		delete(s.refreshTokens, at.RefreshToken)
		return RevokeDone
	}
	if rt, ok := s.refreshTokens[token]; ok {
		if rt.ClientID == "" || rt.ClientID != clientID {
			return RevokeNotOwner
		}
		delete(s.refreshTokens, token)
		delete(s.tokens, rt.AccessToken)
		return RevokeDone
	}
	// An expired access token may already be purged from s.tokens; its refresh
	// token still names it. Linear scan, bounded by maxStoreEntries.
	for k, rt := range s.refreshTokens {
		if rt.AccessToken != "" && rt.AccessToken == token {
			if rt.ClientID == "" || rt.ClientID != clientID {
				return RevokeNotOwner
			}
			delete(s.refreshTokens, k)
			return RevokeDone
		}
	}
	return RevokeUnknown
}

func (s *TokenStore) cleanupExpired() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()

		now := time.Now()

		for state, session := range s.authSessions {
			if now.Sub(session.CreatedAt) > 10*time.Minute {
				delete(s.authSessions, state)
			}
		}

		for code, authCode := range s.authCodes {
			if now.Sub(authCode.CreatedAt) > 10*time.Minute {
				delete(s.authCodes, code)
			}
		}

		for token, data := range s.tokens {
			if now.After(data.ExpiresAt) {
				delete(s.tokens, token)
			}
		}

		for token, data := range s.refreshTokens {
			if now.Sub(data.CreatedAt) > 30*24*time.Hour {
				delete(s.refreshTokens, token)
			}
		}

		s.mu.Unlock()
	}
}
