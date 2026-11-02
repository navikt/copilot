package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DBEntry represents a single row returned from registry_entries.
type DBEntry struct {
	Name  string
	Entry []byte
}

// EntryLoader loads approved server entries from a database or fake.
type EntryLoader func(ctx context.Context) ([]DBEntry, error)

// Store manages server entries merged from static allowlist.json and a database.
type Store struct {
	mu            sync.RWMutex
	allowlistPath string
	config        *Config
	loader        EntryLoader
	hasDBLoaded   bool
	cachedServers []ServerResponse
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
	if staticServers, _, err := s.loadStatic(); err == nil {
		s.cachedServers = staticServers
	}
	return s
}

func (s *Store) loadStatic() ([]ServerResponse, map[string]bool, error) {
	data, err := os.ReadFile(s.allowlistPath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading static allowlist file: %w", err)
	}

	if s.config != nil {
		data = substituteVariables(data, s.config)
	}

	fileInfo, err := os.Stat(s.allowlistPath)
	if err != nil {
		return nil, nil, fmt.Errorf("getting file info for static allowlist: %w", err)
	}

	var staticData StaticRegistryData
	if err := json.Unmarshal(data, &staticData); err != nil {
		return nil, nil, fmt.Errorf("parsing static allowlist JSON: %w", err)
	}

	updatedAt := fileInfo.ModTime().UTC()
	servers := make([]ServerResponse, 0, len(staticData.Servers))
	names := make(map[string]bool, len(staticData.Servers))

	for _, server := range staticData.Servers {
		servers = append(servers, serverToResponse(server, updatedAt))
		names[server.Name] = true
	}

	return servers, names, nil
}

func serverToResponse(s StaticServerData, defaultUpdatedAt time.Time) ServerResponse {
	publishedAt := defaultUpdatedAt
	if s.PublishedAt != "" {
		if parsed, err := time.Parse(time.RFC3339, s.PublishedAt); err == nil {
			publishedAt = parsed
		}
	}
	status := s.Status
	if status == "" {
		status = StatusActive
	}
	return ServerResponse{
		Server: ServerJSON{
			Schema:      CurrentSchemaURL,
			Name:        s.Name,
			Description: s.Description,
			Version:     s.Version,
			WebsiteURL:  s.WebsiteURL,
			Repository:  s.Repository,
			Remotes:     s.Remotes,
			Packages:    s.Packages,
			Meta:        s.Meta,
		},
		Meta: ResponseMeta{
			Official: &RegistryExtensions{
				Status:      status,
				PublishedAt: publishedAt,
				UpdatedAt:   defaultUpdatedAt,
				IsLatest:    true,
			},
			NavRegistry: navRegistryMeta(s),
		},
	}
}

// Reload reloads static entries and approved database entries, merging them.
func (s *Store) Reload(ctx context.Context) error {
	staticServers, staticNames, err := s.loadStatic()
	if err != nil {
		slog.Error("Failed to load static allowlist", "error", err)
		s.mu.RLock()
		hasCache := s.cachedServers != nil
		s.mu.RUnlock()
		if !hasCache {
			return fmt.Errorf("loading static allowlist: %w", err)
		}
	}

	if s.loader == nil {
		if staticServers != nil {
			s.mu.Lock()
			s.cachedServers = staticServers
			s.hasDBLoaded = false
			s.mu.Unlock()
		}
		return nil
	}

	dbEntries, dbErr := s.loader(ctx)
	if dbErr != nil {
		slog.Error("Failed to reload registry entries from database", "error", dbErr)
		s.mu.Lock()
		if !s.hasDBLoaded && staticServers != nil {
			s.cachedServers = staticServers
		}
		s.mu.Unlock()
		return dbErr
	}

	now := time.Now().UTC()
	existingDBNames := make(map[string]bool)
	var validDBResponses []ServerResponse

	for i, entry := range dbEntries {
		rawJSON := entry.Entry
		if s.config != nil {
			rawJSON = substituteVariables(rawJSON, s.config)
		}

		var server StaticServerData
		if err := json.Unmarshal(rawJSON, &server); err != nil {
			slog.Warn("Failed to unmarshal database registry entry, dropping", "server", entry.Name, "error", err)
			continue
		}
		if server.Name == "" {
			server.Name = entry.Name
		}

		if staticNames[server.Name] {
			slog.Warn("Database entry collides with static allowlist name, dropping DB entry", "server", server.Name)
			recordDBCollision(server.Name)
			continue
		}

		if err := validateServerEntry(&server, i, existingDBNames); err != nil {
			slog.Warn("Invalid database registry entry dropped", "server", server.Name, "error", err)
			continue
		}

		existingDBNames[server.Name] = true
		validDBResponses = append(validDBResponses, serverToResponse(server, now))
	}

	merged := make([]ServerResponse, 0, len(staticServers)+len(validDBResponses))
	merged = append(merged, staticServers...)
	merged = append(merged, validDBResponses...)

	s.mu.Lock()
	s.cachedServers = merged
	s.hasDBLoaded = true
	s.mu.Unlock()

	return nil
}

// GetServers returns all merged server responses.
func (s *Store) GetServers() ([]ServerResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cachedServers == nil {
		return nil, errors.New("no servers available in store")
	}
	servers := make([]ServerResponse, len(s.cachedServers))
	copy(servers, s.cachedServers)
	return servers, nil
}

// GetServer returns a specific server response matching name and version.
func (s *Store) GetServer(name, version string) (*ServerResponse, error) {
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
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.Reload(ctx); err != nil {
					slog.Warn("Background reload failed", "error", err)
				}
			}
		}
	}()
}

// newPGXEntryLoader creates an EntryLoader backed by a pgx connection pool.
func newPGXEntryLoader(ctx context.Context, dbURL string) (EntryLoader, func(), error) {
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, nil, fmt.Errorf("creating pgx connection pool: %w", err)
	}

	loader := func(ctx context.Context) ([]DBEntry, error) {
		rows, err := pool.Query(ctx, "SELECT name, entry FROM registry_entries WHERE status = 'approved' ORDER BY name")
		if err != nil {
			return nil, fmt.Errorf("querying registry_entries: %w", err)
		}
		defer rows.Close()

		var entries []DBEntry
		for rows.Next() {
			var e DBEntry
			if err := rows.Scan(&e.Name, &e.Entry); err != nil {
				return nil, fmt.Errorf("scanning registry entry row: %w", err)
			}
			entries = append(entries, e)
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterating registry entry rows: %w", err)
		}
		return entries, nil
	}

	return loader, pool.Close, nil
}
