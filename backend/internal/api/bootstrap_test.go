package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"legacystore/backend/internal/config"
)

func TestBootstrap(t *testing.T) {
	router := NewRouter(config.Config{PublicBaseURL: "http://localhost:8080"}, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bootstrap", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if payload["api_version"] != "v1" {
		t.Fatalf("expected api_version v1, got %v", payload["api_version"])
	}
	if payload["server_status"] != "ok" {
		t.Fatalf("expected server_status ok, got %v", payload["server_status"])
	}
}
