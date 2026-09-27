package main

import (
	"net/http"
	"slices"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// submissions counts answers per survey and status (201, 409, 503, …): the
// signal for alerts on error ratio and on bursts. No identity, ever.
var submissions = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "survey_submissions_total",
	Help: "Survey submissions by survey id and HTTP status.",
}, []string{"survey", "status"})

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// makeRouter wires the probes, the public definitions and the signed-in
// submit route. No ingress: only copilot-cli and my-copilot reach it
// (accessPolicy.inbound).
func makeRouter(auth *authenticator, surveys *surveyAPI) http.Handler {
	mux := http.NewServeMux()
	ok := func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("OK")) }
	mux.HandleFunc("GET /health", ok)
	mux.HandleFunc("GET /ready", ok)
	mux.Handle("GET /metrics", metricsHandler())

	mux.HandleFunc("GET /api/v1/surveys/active", surveys.active)
	submit := authMiddleware(auth, surveys.submit)
	mux.HandleFunc("POST /api/v1/surveys/{id}/responses", func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		submit(rec, r)
		label := "unknown"
		if id := r.PathValue("id"); slices.ContainsFunc(surveys.surveys, func(s survey) bool { return s.ID == id }) {
			label = id
		}
		submissions.WithLabelValues(label, http.StatusText(rec.status)).Inc()
	})
	return mux
}
