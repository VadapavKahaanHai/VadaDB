package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"vadadb/db"
)

func TestWebDatabaseWorkflow(t *testing.T) {
	database, err := db.Open(t.TempDir(), 3)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	server, err := New(database)
	if err != nil {
		t.Fatal(err)
	}
	handler := server.Handler()
	post(t, handler, "/put", url.Values{"key": {"4"}, "value": {"44"}}, http.StatusSeeOther)
	post(t, handler, "/get", url.Values{"key": {"4"}}, http.StatusSeeOther)
	post(t, handler, "/crash", url.Values{"shard": {"1"}}, http.StatusSeeOther)
	post(t, handler, "/get", url.Values{"key": {"4"}}, http.StatusConflict)
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Shard 1 unavailable") || !strings.Contains(page.Body.String(), "Fusion buckets") || !strings.Contains(page.Body.String(), "Storage required") {
		t.Fatalf("GET / = %d: %s", page.Code, page.Body.String())
	}
	post(t, handler, "/recover", url.Values{"shard": {"1"}}, http.StatusSeeOther)
	value, found, err := database.Get(4)
	if err != nil || !found || value != 44 {
		t.Fatalf("recovered value = %d, %v, %v", value, found, err)
	}
	post(t, handler, "/delete", url.Values{"key": {"4"}}, http.StatusSeeOther)
	post(t, handler, "/snapshot", nil, http.StatusSeeOther)
}

func TestWebValidation(t *testing.T) {
	database, _ := db.New(3)
	server, _ := New(database)
	post(t, server.Handler(), "/put", url.Values{"key": {"nope"}, "value": {"1"}}, http.StatusBadRequest)
	post(t, server.Handler(), "/put", url.Values{"key": {"1"}, "value": {"-1"}}, http.StatusBadRequest)
	post(t, server.Handler(), "/crash", url.Values{"shard": {"99"}}, http.StatusBadRequest)
}

func post(t *testing.T, handler http.Handler, path string, values url.Values, want int) {
	t.Helper()
	body := ""
	if values != nil {
		body = values.Encode()
	}
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != want {
		t.Fatalf("POST %s = %d, want %d: %s", path, recorder.Code, want, recorder.Body.String())
	}
}
