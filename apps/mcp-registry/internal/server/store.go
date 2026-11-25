package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/navikt/copilot/mcp-registry/internal/registry"
)

// DBEntry represents a single row returned from registry_entries.
type DBEntry struct {
	Name      string
	Entry     []byte
	UpdatedAt time.Time
}

// dbLoadTimeout bounds one database load, so a stalled query cannot hang a reload.
var dbLoadTimeout = 10 * time.Second

// EntryLoader loads approved server entries from a database or fake.
type EntryLoader func(ctx context.Context) ([]DBEntry, error)

// Store manages server entries merged from static allowlist.json and a database.
// It keeps the last good static set and the last good DB set independently, so
// a failure on one side never drops the other.
type Store struct {
	mu sync.RWMutex
	// reloadMu serialises Reload: the refresher and the post-connect reload
	// overlap, and an older fetch must not apply after a newer one (#2641).
	reloadMu      sync.Mutex
	allowlistPath string
	config        *Config
	loader        EntryLoader
	Fleet         *FleetControlRegistry // nil: static entries come from allowlistPath only
	static        []registry.ServerResponse
	staticNames   map[string]bool
	dbServers     []registry.ServerResponse
	cachedServers []registry.ServerResponse
}

// NewStore creates a new Store instance.
func NewStore(config *Config, allowlistPath string, loader EntryLoader) *Store {
	if allowlistPath == "" {
		allowlistPath = "allowlist.json"
	}
	s := &Store{
		config:        config,
		allowlistPath: allowlistPath,
		loader:        loader,
	}
	if staticServers, names, err := s.loadStatic(); err == nil {
		s.static, s.staticNames, s.cachedServers = staticServers, names, staticServers
	}
	return s
}

func (s *Store) loadStatic() ([]registry.ServerResponse, map[string]bool, error) {
	data, err := os.ReadFile(s.allowlistPath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading static allowlist file: %w", err)
	}
	fileInfo, err := os.Stat(s.allowlistPath)
	if err != nil {
		return nil, nil, fmt.Errorf("getting file info for static allowlist: %w", err)
	}
	return s.parseStatic(data, fileInfo.ModTime().UTC())
}

// parseStatic validates and converts an allowlist document, from the file or
// from a fleet-control snapshot (#2641).
func (s *Store) parseStatic(data []byte, updatedAt time.Time) ([]registry.ServerResponse, map[string]bool, error) {
	// Same validation as startup, so a bad file never replaces the last good set.
	raw, err := registry.DecodeStaticRegistry(data)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing static allowlist JSON: %w", err)
	}
	if err = registry.ValidateRegistry(&raw); err != nil {
		return nil, nil, fmt.Errorf("validating static allowlist: %w", err)
	}

	if s.config != nil {
		data = substituteVariables(data, s.config)
	}

	staticData, err := registry.DecodeStaticRegistry(data)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing static allowlist JSON: %w", err)
	}
	if s.config != nil {
		if err = registry.ValidateSubstitutedRegistry(&raw, &staticData); err != nil {
			return nil, nil, fmt.Errorf("validating static allowlist after substitution: %w", err)
		}
	}

	servers := make([]registry.ServerResponse, 0, len(staticData.Servers))
	names := make(map[string]bool, len(staticData.Servers))

	for _, server := range staticData.Servers {
		servers = append(servers, serverToResponse(server, updatedAt))
		names[server.Name] = true
	}

	return servers, names, nil
}

func serverToResponse(s registry.StaticServerData, defaultUpdatedAt time.Time) registry.ServerResponse {
	publishedAt := defaultUpdatedAt
	if s.PublishedAt != "" {
		if parsed, err := time.Parse(time.RFC3339, s.PublishedAt); err == nil {
			publishedAt = parsed
		}
	}
	status := s.Status
	if status == "" {
		status = registry.StatusActive
	}
	return registry.ServerResponse{
		Server: registry.ServerJSON{
			Schema:      registry.CurrentSchemaURL,
			Name:        s.Name,
			Title:       s.Title,
			Description: s.Description,
			Version:     s.Version,
			WebsiteURL:  s.WebsiteURL,
			Repository:  s.Repository,
			Remotes:     s.Remotes,
			Packages:    s.Packages,
			Meta:        s.Meta,
		},
		Meta: registry.ResponseMeta{
			Official: &registry.Extensions{
				Status:      status,
				PublishedAt: publishedAt,
				UpdatedAt:   defaultUpdatedAt,
				IsLatest:    true,
			},
			NavRegistry: navRegistryMeta(s),
		},
	}
}

// ErrDBNotConnected is returned by the loader until the database connects.
var ErrDBNotConnected = errors.New("database not connected")

// Reload reloads static entries and approved database entries, merging them.
// On a static load error the last good static set is kept; on a DB error the
// last good DB set is kept. The DB error is returned.
func (s *Store) Reload(ctx context.Context) error {
	s.reloadMu.Lock()
	defer s.reloadMu.Unlock()
	var staticServers []registry.ServerResponse
	var staticNames map[string]bool
	var staticErr error
	if s.Fleet != nil {
		staticServers, staticNames, staticErr = s.Fleet.load(ctx, s.parseStatic)
	} else {
		staticServers, staticNames, staticErr = s.loadStatic()
	}
	if staticErr != nil && !errors.Is(staticErr, errNotModified) { // 304 keeps s.static silently
		slog.Error("Failed to load static allowlist, keeping last good set", "error", staticErr)
	}

	var dbServers []registry.ServerResponse
	var dbErr error
	var skipDB bool
	if s.loader != nil {
		loadCtx, cancel := context.WithTimeout(ctx, dbLoadTimeout)
		entries, err := s.loader(loadCtx)
		cancel()
		switch {
		case errors.Is(err, ErrDBNotConnected):
			// The first reload runs before connectWithRetry finishes; the connect path reloads again.
			slog.Debug("Database not connected yet, skipping database entries")
			skipDB = true
		case err != nil:
			slog.Error("Failed to reload registry entries from database, keeping last good set", "error", err)
			dbErr = err
		default:
			dbServers = dbEntriesToResponses(entries)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if staticErr == nil {
		s.static, s.staticNames = staticServers, staticNames
	}
	if s.loader != nil && dbErr == nil && !skipDB {
		s.dbServers = dbServers
	}
	if s.static == nil {
		return fmt.Errorf("loading static allowlist: %w", staticErr)
	}

	merged := make([]registry.ServerResponse, 0, len(s.static)+len(s.dbServers))
	merged = append(merged, s.static...)
	for _, sr := range s.dbServers {
		if s.staticNames[sr.Server.Name] {
			slog.Warn("Database entry collides with static allowlist name, dropping DB entry", "server", sr.Server.Name)
			recordDBDropped("collision")
			continue
		}
		merged = append(merged, sr)
	}
	s.cachedServers = merged
	return dbErr
}

// dbEntriesToResponses validates DB rows with the submission validator and
// converts them. Rows that fail are dropped and counted.
func dbEntriesToResponses(entries []DBEntry) []registry.ServerResponse {
	var out []registry.ServerResponse
	for _, entry := range entries {
		if !json.Valid(entry.Entry) {
			slog.Warn("Failed to parse database registry entry, dropping", "server", entry.Name)
			recordDBDropped("parse")
			continue
		}
		sub, _, err := registry.ValidateSubmission(entry.Entry)
		if err == nil && sub.Name != entry.Name {
			err = fmt.Errorf("entry name %q does not match row name", sub.Name)
		}
		if err != nil {
			slog.Warn("Invalid database registry entry dropped", "server", entry.Name, "error", err)
			recordDBDropped("invalid")
			continue
		}
		remotes := make([]registry.Transport, len(sub.Remotes))
		for i, r := range sub.Remotes {
			remotes[i] = registry.Transport{Type: r.Type, URL: r.URL}
		}
		sr := serverToResponse(registry.StaticServerData{
			Name:        sub.Name,
			Description: sub.Description,
			Version:     sub.Version,
			WebsiteURL:  sub.WebsiteURL,
			Repository:  sub.Repository,
			Remotes:     remotes,
		}, entry.UpdatedAt.UTC())
		sr.Meta.NavRegistry = &registry.NavRegistryMeta{Owner: sub.Owner, Audience: sub.Audience, ProtocolVersions: sub.ProtocolVersions}
		out = append(out, sr)
	}
	return out
}

// GetServers returns all merged server responses.
func (s *Store) GetServers() ([]registry.ServerResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cachedServers == nil {
		return nil, errors.New("no servers available in store")
	}
	servers := make([]registry.ServerResponse, len(s.cachedServers))
	copy(servers, s.cachedServers)
	return servers, nil
}

// GetServer returns a specific server response matching name and version.
func (s *Store) GetServer(name, version string) (*registry.ServerResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cachedServers == nil {
		return nil, errors.New("no servers available in store")
	}
	for _, sr := range s.cachedServers {
		if sr.Server.Name == name && (version == "latest" || sr.Server.Version == version) {
			resp := sr
			return &resp, nil
		}
	}
	return nil, nil
}

// StartRefresher runs a background loop refreshing entries at the given interval.
func (s *Store) StartRefresher(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		s.runRefresher(ctx, ticker.C)
	}()
}

func (s *Store) runRefresher(ctx context.Context, ticks <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			if err := s.Reload(ctx); err != nil {
				slog.Warn("Background reload failed", "error", err)
			}
		}
	}
}
