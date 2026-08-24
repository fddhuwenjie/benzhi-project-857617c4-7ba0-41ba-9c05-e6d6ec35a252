package httpui

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stage-clearance/internal/application"
	"stage-clearance/internal/rules"
	"stage-clearance/internal/store"
)

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	repo, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo, repo, rules.NewDefaultEngine(), nil, nil)
	return NewHandler(service, slog.New(slog.NewTextHandler(io.Discard, nil))).Routes()
}

func TestWorkbenchAndHealth(t *testing.T) {
	handler := testHandler(t)
	for _, path := range []string{"/workbench", "/assets/styles.css", "/assets/app.js", "/api/health"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK || response.Body.Len() == 0 {
			t.Fatalf("GET %s: status=%d size=%d", path, response.Code, response.Body.Len())
		}
	}
}

func TestCreateRequiresActorAndStrictJSON(t *testing.T) {
	handler := testHandler(t)
	body := `{"request_id":"r1","performance_name":"测试","venue_zone":"main","starts_at":"2026-08-25T10:00:00Z","ends_at":"2026-08-25T12:00:00Z"}`
	request := httptest.NewRequest(http.MethodPost, "/api/cases", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("缺少身份状态=%d，期望 422", response.Code)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/cases", strings.NewReader(strings.TrimSuffix(body, "}")+`,"unknown":true}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Actor-Name", "总监")
	request.Header.Set("X-Actor-Role", "technical_director")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("未知 JSON 字段状态=%d，期望 400", response.Code)
	}
}
