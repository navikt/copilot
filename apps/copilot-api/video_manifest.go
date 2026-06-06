package main

import (
	"cloud.google.com/go/storage"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	validVideoID            = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,63}$`)
	validVideoTag           = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)
	validObjectPathRe       = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9/_\-.]*$`)
	videoManifestHTTPClient = &http.Client{Timeout: 10 * time.Second}
)

type VideoMetadata struct {
	Series  string   `json:"series,omitempty"`
	Season  int      `json:"season,omitempty"`
	Episode int      `json:"episode,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

type VideoManifestEntry struct {
	ID              string         `json:"id"`
	Title           string         `json:"title"`
	Description     string         `json:"description"`
	Category        string         `json:"category"`
	PublishedAt     time.Time      `json:"published_at"`
	DurationSec     int            `json:"duration_sec"`
	AspectRatio     string         `json:"aspect_ratio"`
	Language        string         `json:"language"`
	PosterObject    string         `json:"poster_object"`
	HLSMasterObject string         `json:"hls_master_object"`
	MP4Object       string         `json:"mp4_object"`
	CaptionsObject  string         `json:"captions_object"`
	IsPublished     bool           `json:"is_published"`
	SortOrder       int            `json:"sort_order"`
	Metadata        *VideoMetadata `json:"metadata,omitempty"`
}

type VideoFeedItem struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	PublishedAt time.Time      `json:"published_at"`
	DurationSec int            `json:"duration_sec"`
	AspectRatio string         `json:"aspect_ratio"`
	Language    string         `json:"language"`
	PosterURL   string         `json:"poster_url"`
	PlayURL     string         `json:"play_url"`
	MP4URL      string         `json:"mp4_url,omitempty"`
	CaptionsURL string         `json:"captions_url,omitempty"`
	Metadata    *VideoMetadata `json:"metadata,omitempty"`
}

type VideoFeedResponse struct {
	Items      []VideoFeedItem `json:"items"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

type VideoPlayResponse struct {
	ID      string `json:"id"`
	PlayURL string `json:"play_url"`
}

type VideoCaptionsResponse struct {
	ID          string `json:"id"`
	CaptionsURL string `json:"captions_url"`
}

func isValidVideoID(id string) bool {
	return validVideoID.MatchString(id)
}

func validateObjectPath(path string) error {
	if path == "" {
		return fmt.Errorf("object path cannot be empty")
	}
	if strings.HasPrefix(path, "/") || strings.Contains(path, "..") || strings.Contains(path, "//") {
		return fmt.Errorf("invalid object path")
	}
	if strings.Contains(path, "?") || strings.Contains(path, "#") {
		return fmt.Errorf("object path must not contain query or fragment")
	}
	if !validObjectPathRe.MatchString(path) {
		return fmt.Errorf("object path contains invalid characters")
	}
	return nil
}

func validateVideoEntry(e VideoManifestEntry) error {
	if !isValidVideoID(e.ID) {
		return fmt.Errorf("invalid id")
	}
	if e.Title == "" {
		return fmt.Errorf("title is required")
	}
	if e.DurationSec <= 0 {
		return fmt.Errorf("duration_sec must be > 0")
	}
	if e.AspectRatio == "" {
		return fmt.Errorf("aspect_ratio is required")
	}
	if e.PublishedAt.IsZero() {
		return fmt.Errorf("published_at is required")
	}
	if err := validateObjectPath(e.PosterObject); err != nil {
		return fmt.Errorf("poster_object: %w", err)
	}
	if err := validateObjectPath(e.HLSMasterObject); err != nil {
		return fmt.Errorf("hls_master_object: %w", err)
	}
	if e.MP4Object != "" {
		if err := validateObjectPath(e.MP4Object); err != nil {
			return fmt.Errorf("mp4_object: %w", err)
		}
	}
	if e.CaptionsObject != "" {
		if err := validateObjectPath(e.CaptionsObject); err != nil {
			return fmt.Errorf("captions_object: %w", err)
		}
	}
	if e.Metadata != nil {
		if err := validateVideoMetadata(e.Metadata); err != nil {
			return fmt.Errorf("metadata: %w", err)
		}
	}
	return nil
}

func validateVideoMetadata(metadata *VideoMetadata) error {
	series := strings.TrimSpace(metadata.Series)
	if len(series) > 80 {
		return fmt.Errorf("series must be <= 80 characters")
	}
	if metadata.Season < 0 {
		return fmt.Errorf("season must be >= 0")
	}
	if metadata.Episode < 0 {
		return fmt.Errorf("episode must be >= 0")
	}
	if (metadata.Season > 0 && metadata.Episode == 0) || (metadata.Season == 0 && metadata.Episode > 0) {
		return fmt.Errorf("season and episode must be set together")
	}
	if (metadata.Season > 0 || metadata.Episode > 0) && series == "" {
		return fmt.Errorf("series is required when season/episode is set")
	}
	if len(metadata.Tags) > 20 {
		return fmt.Errorf("tags must be <= 20 items")
	}
	seen := make(map[string]struct{}, len(metadata.Tags))
	for _, tag := range metadata.Tags {
		if !validVideoTag.MatchString(tag) {
			return fmt.Errorf("invalid tag %q", tag)
		}
		if _, exists := seen[tag]; exists {
			return fmt.Errorf("duplicate tag %q", tag)
		}
		seen[tag] = struct{}{}
	}
	return nil
}

func objectURL(baseURL, objectPath string) (string, error) {
	if strings.TrimSpace(baseURL) == "" {
		return "", fmt.Errorf("base URL is not configured")
	}
	if err := validateObjectPath(objectPath); err != nil {
		return "", err
	}
	return strings.TrimRight(baseURL, "/") + "/" + objectPath, nil
}

type videoManifestCache struct {
	mu            sync.Mutex
	source        string
	ttl           time.Duration
	now           func() time.Time
	loader        func(context.Context, string) ([]VideoManifestEntry, error)
	cachedEntries []VideoManifestEntry
	loaded        bool
	expiresAt     time.Time
}

func newVideoManifestCache(source string, ttl time.Duration) *videoManifestCache {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &videoManifestCache{
		source: source,
		ttl:    ttl,
		now:    time.Now,
		loader: loadVideoManifestFromSource,
	}
}

func (c *videoManifestCache) get(ctx context.Context) ([]VideoManifestEntry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.loaded && c.now().Before(c.expiresAt) {
		return cloneVideoManifestEntries(c.cachedEntries), nil
	}

	entries, err := c.loader(ctx, c.source)
	if err != nil {
		if c.loaded {
			return cloneVideoManifestEntries(c.cachedEntries), fmt.Errorf("refreshing manifest: %w", err)
		}
		return nil, err
	}

	c.cachedEntries = cloneVideoManifestEntries(entries)
	c.loaded = true
	c.expiresAt = c.now().Add(c.ttl)
	return cloneVideoManifestEntries(c.cachedEntries), nil
}

func cloneVideoManifestEntries(entries []VideoManifestEntry) []VideoManifestEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]VideoManifestEntry, len(entries))
	copy(out, entries)
	return out
}

func loadVideoManifestFromSource(ctx context.Context, source string) ([]VideoManifestEntry, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, fmt.Errorf("manifest source is not configured")
	}

	raw, err := readManifestSource(ctx, source)
	if err != nil {
		return nil, err
	}

	var entries []VideoManifestEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("failed parsing manifest: %w", err)
	}

	for _, e := range entries {
		if err := validateVideoEntry(e); err != nil {
			return nil, fmt.Errorf("invalid manifest entry %q: %w", e.ID, err)
		}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].SortOrder == entries[j].SortOrder {
			return entries[i].PublishedAt.After(entries[j].PublishedAt)
		}
		return entries[i].SortOrder < entries[j].SortOrder
	})

	return entries, nil
}

func readManifestSource(ctx context.Context, source string) ([]byte, error) {
	switch {
	case strings.HasPrefix(source, "gs://"):
		return readManifestGCS(ctx, source)
	case strings.HasPrefix(source, "http://"), strings.HasPrefix(source, "https://"):
		return readManifestURL(ctx, source)
	default:
		raw, err := os.ReadFile(source)
		if err != nil {
			return nil, fmt.Errorf("failed reading manifest: %w", err)
		}
		return raw, nil
	}
}

func readManifestURL(ctx context.Context, manifestURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating manifest request: %w", err)
	}
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")

	resp, err := videoManifestHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching manifest: unexpected status %s", resp.Status)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading manifest response: %w", err)
	}
	return raw, nil
}

func readManifestGCS(ctx context.Context, source string) (raw []byte, err error) {
	source = strings.TrimPrefix(source, "gs://")
	bucket, object, found := strings.Cut(source, "/")
	if !found || bucket == "" || object == "" {
		return nil, fmt.Errorf("invalid gs:// manifest source: %s", source)
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating GCS client: %w", err)
	}
	defer func() {
		if cerr := client.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("closing GCS client: %w", cerr)
		}
	}()

	rc, err := client.Bucket(bucket).Object(object).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("opening manifest object gs://%s/%s: %w", bucket, object, err)
	}
	defer func() {
		if cerr := rc.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("closing manifest reader gs://%s/%s: %w", bucket, object, cerr)
		}
	}()

	raw, err = io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("reading manifest object gs://%s/%s: %w", bucket, object, err)
	}
	return raw, nil
}
