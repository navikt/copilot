package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testVideoManifestJSON() []byte {
	return []byte(`[
  {
    "id": "intro-cli",
    "title": "Intro til Copilot CLI",
    "description": "Kort intro",
    "category": "copilot",
    "published_at": "2026-06-01T10:00:00Z",
    "duration_sec": 42,
    "aspect_ratio": "9:16",
    "language": "nb",
    "poster_object": "videos/intro-cli/poster.jpg",
    "hls_master_object": "videos/intro-cli/master.m3u8",
    "mp4_object": "videos/intro-cli/video.mp4",
    "captions_object": "videos/intro-cli/captions.vtt",
    "is_published": true,
    "sort_order": 1,
    "metadata": {
      "series": "kost-optimalisering",
      "season": 1,
      "episode": 1,
      "tags": ["prompting", "cost"]
    }
  },
  {
    "id": "draft-video",
    "title": "Draft",
    "description": "Ikke publisert",
    "category": "copilot",
    "published_at": "2026-06-02T10:00:00Z",
    "duration_sec": 30,
    "aspect_ratio": "9:16",
    "language": "nb",
    "poster_object": "videos/draft/poster.jpg",
    "hls_master_object": "videos/draft/master.m3u8",
    "captions_object": "",
    "is_published": false,
    "sort_order": 2
  }
]`)
}

func newTestVideoManifestServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(testVideoManifestJSON()); err != nil {
			t.Fatalf("failed writing manifest response: %v", err)
		}
	}))

	t.Cleanup(server.Close)
	return server, &hits
}

func newTestVideoRouter(t *testing.T) http.Handler {
	t.Helper()
	server, _ := newTestVideoManifestServer(t)

	cfg := &Config{
		VideoManifestURL:      server.URL,
		VideoBucketPublic:     "copilot-videos-public",
		VideoFeedCacheSeconds: 60,
	}

	return makePublicRouter(cfg, newVideoHandlers(cfg))
}

func newTestVideoRouterWithPlayLimit(t *testing.T, limit int) http.Handler {
	t.Helper()
	server, _ := newTestVideoManifestServer(t)

	cfg := &Config{
		VideoManifestURL:      server.URL,
		VideoBucketPublic:     "copilot-videos-public",
		VideoFeedCacheSeconds: 60,
	}
	videoHandlers := newVideoHandlers(cfg)
	videoHandlers.playRateLimiter = newVideoPlayRateLimiter(limit, time.Minute)
	return makePublicRouter(cfg, videoHandlers)
}

func TestLoadVideoManifestFromHTTP(t *testing.T) {
	server, _ := newTestVideoManifestServer(t)

	entries, err := loadVideoManifestFromSource(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("loadVideoManifestFromSource returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].ID != "intro-cli" {
		t.Fatalf("expected published item first after sort, got %q", entries[0].ID)
	}
}

func TestVideoManifestCacheUsesTTL(t *testing.T) {
	cache := newVideoManifestCache("manifest-source", time.Minute)

	current := time.Unix(0, 0)
	var loads int
	cache.now = func() time.Time { return current }
	cache.loader = func(context.Context, string) ([]VideoManifestEntry, error) {
		loads++
		if loads == 1 {
			return []VideoManifestEntry{{ID: "intro-cli", Title: "Intro", DurationSec: 42, AspectRatio: "9:16", PublishedAt: current.Add(time.Minute), PosterObject: "poster.jpg", HLSMasterObject: "master.m3u8"}}, nil
		}
		return []VideoManifestEntry{{ID: "new-video", Title: "New", DurationSec: 42, AspectRatio: "9:16", PublishedAt: current.Add(2 * time.Minute), PosterObject: "poster.jpg", HLSMasterObject: "master.m3u8"}}, nil
	}

	first, err := cache.get(context.Background())
	if err != nil {
		t.Fatalf("first cache load failed: %v", err)
	}
	second, err := cache.get(context.Background())
	if err != nil {
		t.Fatalf("second cache read failed: %v", err)
	}
	if loads != 1 {
		t.Fatalf("expected one load within TTL, got %d", loads)
	}
	if first[0].ID != second[0].ID {
		t.Fatalf("expected cached entry to match, got %q and %q", first[0].ID, second[0].ID)
	}

	current = current.Add(time.Minute + time.Second)
	third, err := cache.get(context.Background())
	if err != nil {
		t.Fatalf("third cache load failed: %v", err)
	}
	if loads != 2 {
		t.Fatalf("expected reload after TTL, got %d loads", loads)
	}
	if third[0].ID != "new-video" {
		t.Fatalf("expected refreshed manifest, got %q", third[0].ID)
	}
}

func TestVideoFeedEndpoint(t *testing.T) {
	router := newTestVideoRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/public/v1/videos?limit=10", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"id":"intro-cli"`) {
		t.Fatalf("expected published video in feed, got %s", body)
	}
	if !strings.Contains(body, `"play_url":"https://storage.googleapis.com/copilot-videos-public/videos/intro-cli/master.m3u8"`) {
		t.Fatalf("expected play_url in feed, got %s", body)
	}
	if !strings.Contains(body, `"mp4_url":"https://storage.googleapis.com/copilot-videos-public/videos/intro-cli/video.mp4"`) {
		t.Fatalf("expected mp4_url in feed, got %s", body)
	}
	if !strings.Contains(body, `"captions_url":"https://storage.googleapis.com/copilot-videos-public/videos/intro-cli/captions.vtt"`) {
		t.Fatalf("expected captions_url in feed, got %s", body)
	}
	if !strings.Contains(body, `"metadata":{"series":"kost-optimalisering","season":1,"episode":1,"tags":["prompting","cost"]}`) {
		t.Fatalf("expected metadata in feed, got %s", body)
	}
	if strings.Contains(body, `"id":"draft-video"`) {
		t.Fatalf("did not expect unpublished video in feed, got %s", body)
	}
}

func TestVideoFeedEndpointServesStaleManifestOnRefreshError(t *testing.T) {
	server, _ := newTestVideoManifestServer(t)
	cfg := &Config{
		VideoManifestURL:      server.URL,
		VideoBucketPublic:     "copilot-videos-public",
		VideoFeedCacheSeconds: 60,
	}
	videoHandlers := newVideoHandlers(cfg)

	current := time.Unix(0, 0)
	var loads int
	videoHandlers.manifestCache.now = func() time.Time { return current }
	videoHandlers.manifestCache.loader = func(context.Context, string) ([]VideoManifestEntry, error) {
		loads++
		if loads == 1 {
			return []VideoManifestEntry{{
				ID:              "intro-cli",
				Title:           "Intro",
				DurationSec:     42,
				AspectRatio:     "9:16",
				PublishedAt:     current.Add(time.Minute),
				PosterObject:    "videos/intro-cli/poster.jpg",
				HLSMasterObject: "videos/intro-cli/master.m3u8",
				IsPublished:     true,
			}}, nil
		}
		return []VideoManifestEntry{{
			ID:              "new-video",
			Title:           "New",
			DurationSec:     42,
			AspectRatio:     "9:16",
			PublishedAt:     current.Add(2 * time.Minute),
			PosterObject:    "videos/new-video/poster.jpg",
			HLSMasterObject: "videos/new-video/master.m3u8",
			IsPublished:     true,
		}}, fmt.Errorf("temporary manifest refresh failure")
	}

	router := makePublicRouter(cfg, videoHandlers)
	reqWarm := httptest.NewRequest(http.MethodGet, "/public/v1/videos?limit=10", nil)
	recWarm := httptest.NewRecorder()
	router.ServeHTTP(recWarm, reqWarm)
	if recWarm.Code != http.StatusOK {
		t.Fatalf("expected warm cache request to succeed, got %d", recWarm.Code)
	}

	current = current.Add(time.Minute + time.Second)
	req := httptest.NewRequest(http.MethodGet, "/public/v1/videos?limit=10", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected stale response to stay 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"id":"intro-cli"`) {
		t.Fatalf("expected stale video in feed, got %s", body)
	}
	if strings.Contains(body, `"id":"new-video"`) {
		t.Fatalf("did not expect refreshed video after manifest error, got %s", body)
	}
}

func TestVideoPlayEndpoint(t *testing.T) {
	router := newTestVideoRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/public/v1/videos/intro-cli/play", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"play_url":"https://storage.googleapis.com/copilot-videos-public/videos/intro-cli/master.m3u8"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestVideoCaptionsEndpoint(t *testing.T) {
	router := newTestVideoRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/public/v1/videos/intro-cli/captions", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"captions_url":"https://storage.googleapis.com/copilot-videos-public/videos/intro-cli/captions.vtt"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestVideoPlayInvalidID(t *testing.T) {
	router := newTestVideoRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/public/v1/videos/INVALID/play", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestVideoFeedInvalidCursor(t *testing.T) {
	router := newTestVideoRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/public/v1/videos?cursor=bad", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestVideoPlayRateLimitExceeded(t *testing.T) {
	router := newTestVideoRouterWithPlayLimit(t, 2)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/public/v1/videos/intro-cli/play", nil)
		req.RemoteAddr = "203.0.113.10:1234"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if i < 2 && rec.Code != http.StatusOK {
			t.Fatalf("expected 200 before limit, got %d", rec.Code)
		}
		if i == 2 {
			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf("expected 429 after limit, got %d", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), `"status":429`) {
				t.Fatalf("expected RFC7807 status in body, got %s", rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "/errors/rate_limited") {
				t.Fatalf("expected rate_limited error type, got %s", rec.Body.String())
			}
		}
	}
}

func TestVideoPlayRateLimitPerClient(t *testing.T) {
	router := newTestVideoRouterWithPlayLimit(t, 1)

	reqA := httptest.NewRequest(http.MethodGet, "/public/v1/videos/intro-cli/play", nil)
	reqA.RemoteAddr = "203.0.113.10:1234"
	recA := httptest.NewRecorder()
	router.ServeHTTP(recA, reqA)
	if recA.Code != http.StatusOK {
		t.Fatalf("expected 200 for first client, got %d", recA.Code)
	}

	reqB := httptest.NewRequest(http.MethodGet, "/public/v1/videos/intro-cli/play", nil)
	reqB.RemoteAddr = "203.0.113.11:1234"
	recB := httptest.NewRecorder()
	router.ServeHTTP(recB, reqB)
	if recB.Code != http.StatusOK {
		t.Fatalf("expected 200 for second client, got %d", recB.Code)
	}
}

func TestLoadVideoManifestFromSourceRejectsInvalidPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"invalid":true}`))
	}))
	t.Cleanup(server.Close)

	if _, err := loadVideoManifestFromSource(context.Background(), server.URL); err == nil {
		t.Fatal("expected error for invalid manifest payload")
	}
}

func TestLoadVideoManifestFromSourceRejectsInvalidMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
      {
        "id": "intro-cli",
        "title": "Intro",
        "description": "",
        "category": "copilot",
        "published_at": "2026-06-01T10:00:00Z",
        "duration_sec": 42,
        "aspect_ratio": "9:16",
        "language": "nb",
        "poster_object": "videos/intro-cli/poster.jpg",
        "hls_master_object": "videos/intro-cli/master.m3u8",
        "captions_object": "",
        "is_published": true,
        "sort_order": 1,
        "metadata": {
          "season": 1
        }
      }
    ]`))
	}))
	t.Cleanup(server.Close)

	if _, err := loadVideoManifestFromSource(context.Background(), server.URL); err == nil {
		t.Fatal("expected error for invalid metadata payload")
	}
}
