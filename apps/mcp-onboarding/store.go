package main

import (
	"errors"
	"sync"
	"time"
)

var ErrNotFound = errors.New("not found")

// TODO: If we need to scale beyond a single replica, replace in-memory maps with Redis
// to share sessions and tokens across pods. Client registrations are not here:
// the client_id carries its own signed registration (see clientid.go).

type AuthSession struct {
	ClientID            string
	ClientState         string
	RedirectURI         string
	CodeChallenge       string
	CodeChallengeMethod string
	CreatedAt           time.Time
}

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

type TokenData struct {
	GitHubAccessToken  string
	GitHubRefreshToken string
	GitHubExpiresAt    time.Time
	UserLogin          string
	UserID             int64
	ExpiresAt          time.Time
}

type RefreshTokenData struct {
	// ClientID is the client the token was issued to. The refresh grant
	// requires the caller to present it, so a stolen refresh token is not
	// usable on its own (GHSA-7hwf-488h-59x8).
	ClientID           string
	GitHubRefreshToken string
	UserLogin          string
	UserID             int64
	CreatedAt          time.Time
}

type TokenStore struct {
	authSessions  map[string]*AuthSession
	authCodes     map[string]*AuthCode
	tokens        map[string]*TokenData
	refreshTokens map[string]*RefreshTokenData
	mu            sync.RWMutex
}

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

func (s *TokenStore) SaveAuthSession(state string, session *AuthSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authSessions[state] = session
}

func (s *TokenStore) GetAuthSession(state string) (*AuthSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.authSessions[state]
	if !ok {
		return nil, ErrNotFound
	}
	return session, nil
}

func (s *TokenStore) DeleteAuthSession(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.authSessions, state)
}

func (s *TokenStore) SaveAuthCode(code string, authCode *AuthCode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authCodes[code] = authCode
}

func (s *TokenStore) GetAuthCode(code string) (*AuthCode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	authCode, ok := s.authCodes[code]
	if !ok {
		return nil, ErrNotFound
	}
	return authCode, nil
}

func (s *TokenStore) DeleteAuthCode(code string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.authCodes, code)
}

func (s *TokenStore) SaveToken(token string, data *TokenData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token] = data
}

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

func (s *TokenStore) DeleteToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, token)
}

func (s *TokenStore) SaveRefreshToken(token string, data *RefreshTokenData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshTokens[token] = data
}

func (s *TokenStore) GetRefreshToken(token string) (*RefreshTokenData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.refreshTokens[token]
	if !ok {
		return nil, ErrNotFound
	}
	return data, nil
}

func (s *TokenStore) DeleteRefreshToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.refreshTokens, token)
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
