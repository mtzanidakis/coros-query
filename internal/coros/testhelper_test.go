package coros

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockHandler lets a test register a handler per path. Unknown paths return 404.
type mockHandler struct {
	t        *testing.T
	handlers map[string]http.HandlerFunc
	calls    map[string]int
}

func newMock(t *testing.T) *mockHandler {
	t.Helper()
	return &mockHandler{
		t:        t,
		handlers: map[string]http.HandlerFunc{},
		calls:    map[string]int{},
	}
}

func (m *mockHandler) on(path string, fn http.HandlerFunc) *mockHandler {
	m.handlers[path] = fn
	return m
}

func (m *mockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.calls[r.URL.Path]++
	if fn, ok := m.handlers[r.URL.Path]; ok {
		fn(w, r)
		return
	}
	m.t.Errorf("unexpected request to %s", r.URL.Path)
	http.NotFound(w, r)
}

// envelope writes a Coros response envelope {apiCode, message, result, data}.
func envelope(w http.ResponseWriter, result string, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"apiCode": "TEST",
		"message": "OK",
		"result":  result,
		"data":    data,
	})
}

// newTestClient spins up a Client pointed at a httptest server with a
// temporary cache dir. The client is pre-populated with a fake token so tests
// for read endpoints don't need to stub /account/login.
func newTestClient(t *testing.T, mock http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(mock)
	t.Cleanup(ts.Close)
	c, err := New(Config{
		Region:   "test",
		Email:    "user@example.com",
		Password: "pw",
		UserID:   "u42",
		BaseURL:  ts.URL,
		CacheDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.token = "test-token"
	return c, ts
}
