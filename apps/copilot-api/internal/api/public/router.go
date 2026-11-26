package public

import (
	"net/http"

	"github.com/navikt/copilot/apps/copilot-api/internal/api/httpx"
	"github.com/navikt/copilot/apps/copilot-api/internal/config"
)

// Router creates the public router for unauthenticated endpoints.
func Router(config *config.Config, videoHandlers *VideoHandlers, publicHandlers *Handlers) http.Handler {
	mux := http.NewServeMux()

	videoStub := httpx.ServiceUnavailableHandler("Video service is not configured for this environment")
	video := func(h http.HandlerFunc) http.HandlerFunc {
		if videoHandlers != nil {
			return h
		}
		return videoStub
	}

	mux.HandleFunc("GET /public/v1/videos", video(httpx.NilSafe(videoHandlers, func(h *VideoHandlers) http.HandlerFunc { return h.handleVideoFeed })))
	mux.HandleFunc("GET /public/v1/videos/{id}", video(httpx.NilSafe(videoHandlers, func(h *VideoHandlers) http.HandlerFunc { return h.handleVideoDetail })))
	mux.HandleFunc("GET /public/v1/videos/{id}/play", video(httpx.NilSafe(videoHandlers, func(h *VideoHandlers) http.HandlerFunc { return h.handleVideoPlay })))
	mux.HandleFunc("GET /public/v1/videos/{id}/captions", video(httpx.NilSafe(videoHandlers, func(h *VideoHandlers) http.HandlerFunc { return h.handleVideoCaptions })))
	fact := httpx.ServiceUnavailableHandler("BigQuery is not configured for this environment")
	if publicHandlers != nil {
		fact = publicHandlers.HandlePublicFact
	}
	mux.HandleFunc("GET /public/v1/fact", fact)

	return mux
}
