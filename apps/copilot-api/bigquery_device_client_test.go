package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	bqstore "github.com/navikt/copilot/apps/copilot-api/internal/bigquery"
)

func (m *mockBigQueryClient) GetEditorUsage(context.Context) ([]bqstore.EditorUsage, error) {
	return m.editorUsage, m.editorUsageErr
}

func TestEditorUsageEmptyHandler(t *testing.T) {
	h := &BigQueryHandlers{bqClient: &mockBigQueryClient{}}
	w := httptest.NewRecorder()
	h.handleEditorUsage(w, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/editors", nil))
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("response: %d %s", w.Code, w.Body.String())
	}
}

func TestEditorUsageHandlerError(t *testing.T) {
	h := &BigQueryHandlers{bqClient: &mockBigQueryClient{editorUsageErr: errors.New("unavailable")}}
	w := httptest.NewRecorder()
	h.handleEditorUsage(w, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/editors", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}
