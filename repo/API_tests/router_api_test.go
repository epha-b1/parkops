package API_tests

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestLoginPageRenders(t *testing.T) {
	env := setupAuthAPIEnv(t)
	w := apiRequest(t, env.r, http.MethodGet, "/login", nil, nil)
	logStep(t, "GET", "/login", w.Code, w.Body.String())

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Fatalf("expected html content type, got %s", contentType)
	}

	body, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	if !strings.Contains(string(body), "ParkOps") {
		t.Fatal("expected login page to contain ParkOps")
	}
}

func TestAPINotFoundUsesStandardErrorShape(t *testing.T) {
	env := setupAuthAPIEnv(t)
	w := apiRequest(t, env.r, http.MethodGet, "/api/does-not-exist", nil, nil)
	logStep(t, "GET", "/api/does-not-exist", w.Code, w.Body.String())

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, `"code":"NOT_FOUND"`) {
		t.Fatalf("expected error code in body, got %s", body)
	}
	if !strings.Contains(body, `"message":"resource not found"`) {
		t.Fatalf("expected error message in body, got %s", body)
	}
}
