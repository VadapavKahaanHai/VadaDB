package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"vadadb/db"
)

func TestAPIWorkflow(t *testing.T) {
	database, err := db.New(3)
	if err != nil {
		t.Fatal(err)
	}
	handler := New(database)

	assertStatus(t, handler, http.MethodPut, "/kv/4", `{"value":44}`, http.StatusOK)
	assertStatus(t, handler, http.MethodPut, "/kv/5", `{"value":55}`, http.StatusOK)
	get := perform(handler, http.MethodGet, "/kv/4", "")
	if get.Code != http.StatusOK || !bytes.Contains(get.Body.Bytes(), []byte(`"value":44`)) {
		t.Fatalf("GET response: %d %s", get.Code, get.Body.String())
	}
	assertStatus(t, handler, http.MethodGet, "/kv/99", "", http.StatusNotFound)
	assertStatus(t, handler, http.MethodGet, "/scan", "", http.StatusOK)
	assertStatus(t, handler, http.MethodPost, "/admin/shards/1/crash", "", http.StatusOK)
	assertStatus(t, handler, http.MethodGet, "/kv/4", "", http.StatusServiceUnavailable)
	assertStatus(t, handler, http.MethodPost, "/admin/shards/1/recover", "", http.StatusOK)
	assertStatus(t, handler, http.MethodGet, "/kv/4", "", http.StatusOK)
	assertStatus(t, handler, http.MethodDelete, "/kv/4", "", http.StatusNoContent)
	assertStatus(t, handler, http.MethodGet, "/kv/4", "", http.StatusNotFound)
	assertStatus(t, handler, http.MethodGet, "/admin/storage", "", http.StatusOK)
}

func TestAPIValidationAndShardInspection(t *testing.T) {
	database, err := db.New(3)
	if err != nil {
		t.Fatal(err)
	}
	handler := New(database)
	assertStatus(t, handler, http.MethodPut, "/kv/nope", `{"value":1}`, http.StatusBadRequest)
	assertStatus(t, handler, http.MethodPut, "/kv/1", `{}`, http.StatusBadRequest)
	assertStatus(t, handler, http.MethodPut, "/kv/1", `{"value":1,"extra":2}`, http.StatusBadRequest)
	assertStatus(t, handler, http.MethodPost, "/scan", "", http.StatusMethodNotAllowed)
	response := perform(handler, http.MethodGet, "/admin/shards", "")
	if response.Code != http.StatusOK {
		t.Fatalf("shards response: %d %s", response.Code, response.Body.String())
	}
	var shards []db.ShardInfo
	if err := json.NewDecoder(response.Body).Decode(&shards); err != nil || len(shards) != 3 {
		t.Fatalf("shards = %#v, err = %v", shards, err)
	}
}

func perform(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func assertStatus(t *testing.T, handler http.Handler, method, path, body string, want int) {
	t.Helper()
	response := perform(handler, method, path, body)
	if response.Code != want {
		t.Fatalf("%s %s: got %d, want %d: %s", method, path, response.Code, want, response.Body.String())
	}
}
