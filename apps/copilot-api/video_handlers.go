package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type VideoHandlers struct {
	manifestCache    *videoManifestCache
	publicBaseURL    string
	feedCacheSeconds int
	playRateLimiter  *videoPlayRateLimiter
}

func newVideoHandlers(config *Config) *VideoHandlers {
	manifestSource := strings.TrimSpace(config.VideoManifestURL)
	if manifestSource == "" {
		manifestSource = strings.TrimSpace(config.VideoManifestPath)
	}
	if manifestSource == "" && config.VideoBucketPublic != "" {
		manifestSource = fmt.Sprintf("https://storage.googleapis.com/%s/video_manifest.json", config.VideoBucketPublic)
	}
	if manifestSource == "" {
		manifestSource = "video_manifest.json"
	}

	baseURL := strings.TrimSpace(config.VideoPublicBaseURL)
	if baseURL == "" && config.VideoBucketPublic != "" {
		baseURL = fmt.Sprintf("https://storage.googleapis.com/%s", config.VideoBucketPublic)
	}

	return &VideoHandlers{
		manifestCache:    newVideoManifestCache(manifestSource, time.Duration(config.VideoFeedCacheSeconds)*time.Second),
		publicBaseURL:    strings.TrimRight(baseURL, "/"),
		feedCacheSeconds: config.VideoFeedCacheSeconds,
		playRateLimiter:  newVideoPlayRateLimiter(60, time.Minute),
	}
}

type videoPlayRateState struct {
	count   int
	resetAt time.Time
}

type videoPlayRateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	clients map[string]videoPlayRateState
}

func newVideoPlayRateLimiter(limit int, window time.Duration) *videoPlayRateLimiter {
	return &videoPlayRateLimiter{
		limit:   limit,
		window:  window,
		clients: make(map[string]videoPlayRateState),
	}
}

func (l *videoPlayRateLimiter) allow(clientKey string) bool {
	if l == nil {
		return true
	}
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	state, ok := l.clients[clientKey]
	if !ok || !now.Before(state.resetAt) {
		l.clients[clientKey] = videoPlayRateState{
			count:   1,
			resetAt: now.Add(l.window),
		}
		return true
	}

	if state.count >= l.limit {
		return false
	}

	state.count++
	l.clients[clientKey] = state
	return true
}

func videoPlayClientKey(r *http.Request) string {
	remoteAddr := strings.TrimSpace(r.RemoteAddr)
	if remoteAddr == "" {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil && host != "" {
		return host
	}
	return remoteAddr
}

func (h *VideoHandlers) handleVideoFeed(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	entries, err := h.manifestCache.get(r.Context())
	if err != nil {
		if len(entries) == 0 {
			slog.Error("Failed to load video manifest", "error", err)
			respondError(w, "service_unavailable", "Video feed is unavailable", http.StatusServiceUnavailable)
			return
		}
		slog.Warn("Serving stale video manifest after refresh error", "error", err)
	}

	limit := 10
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		n, err := strconv.Atoi(rawLimit)
		if err != nil || n < 1 || n > 50 {
			respondError(w, "invalid_parameter", "limit must be between 1 and 50", http.StatusBadRequest)
			return
		}
		limit = n
	}

	offset := 0
	if cursor := r.URL.Query().Get("cursor"); cursor != "" {
		n, err := strconv.Atoi(cursor)
		if err != nil || n < 0 {
			respondError(w, "invalid_parameter", "cursor must be a non-negative integer", http.StatusBadRequest)
			return
		}
		offset = n
	}

	published := make([]VideoManifestEntry, 0, len(entries))
	for _, e := range entries {
		if e.IsPublished {
			published = append(published, e)
		}
	}

	if offset > len(published) {
		respondError(w, "invalid_parameter", "cursor out of range", http.StatusBadRequest)
		return
	}

	end := offset + limit
	if end > len(published) {
		end = len(published)
	}

	items := make([]VideoFeedItem, 0, end-offset)
	for _, e := range published[offset:end] {
		posterURL, err := objectURL(h.publicBaseURL, e.PosterObject)
		if err != nil {
			slog.Error("Invalid poster object in manifest", "id", e.ID, "error", err)
			respondError(w, "internal_error", "Invalid video manifest configuration", http.StatusInternalServerError)
			return
		}
		playURL, err := objectURL(h.publicBaseURL, e.HLSMasterObject)
		if err != nil {
			slog.Error("Invalid HLS object in manifest", "id", e.ID, "error", err)
			respondError(w, "internal_error", "Invalid video manifest configuration", http.StatusInternalServerError)
			return
		}
		var mp4URL string
		if e.MP4Object != "" {
			mp4URL, err = objectURL(h.publicBaseURL, e.MP4Object)
			if err != nil {
				slog.Error("Invalid MP4 object in manifest", "id", e.ID, "error", err)
				respondError(w, "internal_error", "Invalid video manifest configuration", http.StatusInternalServerError)
				return
			}
		}
		var captionsURL string
		if e.CaptionsObject != "" {
			captionsURL, err = objectURL(h.publicBaseURL, e.CaptionsObject)
			if err != nil {
				slog.Error("Invalid captions object in manifest", "id", e.ID, "error", err)
				respondError(w, "internal_error", "Invalid video manifest configuration", http.StatusInternalServerError)
				return
			}
		}
		items = append(items, VideoFeedItem{
			ID:          e.ID,
			Title:       e.Title,
			Description: e.Description,
			Category:    e.Category,
			PublishedAt: e.PublishedAt,
			DurationSec: e.DurationSec,
			AspectRatio: e.AspectRatio,
			Language:    e.Language,
			PosterURL:   posterURL,
			PlayURL:     playURL,
			MP4URL:      mp4URL,
			CaptionsURL: captionsURL,
		})
	}

	resp := VideoFeedResponse{Items: items}
	if end < len(published) {
		resp.NextCursor = strconv.Itoa(end)
	}

	cacheSeconds := h.feedCacheSeconds
	if cacheSeconds <= 0 {
		cacheSeconds = 60
	}
	cacheControl(w, cacheSeconds, true)
	respondJSON(w, resp, http.StatusOK)
}

func (h *VideoHandlers) handleVideoPlay(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if !h.playRateLimiter.allow(videoPlayClientKey(r)) {
		respondError(w, "rate_limited", "Too many play requests", http.StatusTooManyRequests)
		return
	}

	id := r.PathValue("id")
	if !isValidVideoID(id) {
		respondError(w, "invalid_parameter", "Invalid video id", http.StatusBadRequest)
		return
	}

	entry, err := h.getPublishedVideoByID(r.Context(), id)
	if err != nil {
		slog.Error("Failed to load video by id", "id", id, "error", err)
		respondError(w, "service_unavailable", "Video playback is unavailable", http.StatusServiceUnavailable)
		return
	}
	if entry == nil {
		respondError(w, "not_found", "Video not found", http.StatusNotFound)
		return
	}

	playURL, err := objectURL(h.publicBaseURL, entry.HLSMasterObject)
	if err != nil {
		slog.Error("Invalid HLS object in manifest", "id", id, "error", err)
		respondError(w, "internal_error", "Invalid video manifest configuration", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 300, true)
	respondJSON(w, VideoPlayResponse{
		ID:      id,
		PlayURL: playURL,
	}, http.StatusOK)
}

func (h *VideoHandlers) handleVideoCaptions(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	id := r.PathValue("id")
	if !isValidVideoID(id) {
		respondError(w, "invalid_parameter", "Invalid video id", http.StatusBadRequest)
		return
	}

	entry, err := h.getPublishedVideoByID(r.Context(), id)
	if err != nil {
		slog.Error("Failed to load captions by id", "id", id, "error", err)
		respondError(w, "service_unavailable", "Video captions are unavailable", http.StatusServiceUnavailable)
		return
	}
	if entry == nil {
		respondError(w, "not_found", "Video not found", http.StatusNotFound)
		return
	}
	if entry.CaptionsObject == "" {
		respondError(w, "not_found", "Captions not found for this video", http.StatusNotFound)
		return
	}

	captionsURL, err := objectURL(h.publicBaseURL, entry.CaptionsObject)
	if err != nil {
		slog.Error("Invalid captions object in manifest", "id", id, "error", err)
		respondError(w, "internal_error", "Invalid video manifest configuration", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 300, true)
	respondJSON(w, VideoCaptionsResponse{
		ID:          id,
		CaptionsURL: captionsURL,
	}, http.StatusOK)
}

func (h *VideoHandlers) getPublishedVideoByID(ctx context.Context, id string) (*VideoManifestEntry, error) {
	entries, err := h.manifestCache.get(ctx)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.ID == id && e.IsPublished {
			entry := e
			return &entry, nil
		}
	}
	return nil, nil
}
