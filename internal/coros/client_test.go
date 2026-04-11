package coros

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestLogin_SuccessPopulatesTokenAndUserID(t *testing.T) {
	mock := newMock(t).on("/account/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		// md5 of "pw"
		sum := md5.Sum([]byte("pw"))
		want := hex.EncodeToString(sum[:])
		if body["pwd"] != want {
			t.Errorf("pwd = %v, want md5(pw)=%s", body["pwd"], want)
		}
		if body["account"] != "user@example.com" {
			t.Errorf("account = %v", body["account"])
		}
		if got := body["accountType"]; got != float64(2) {
			t.Errorf("accountType = %v, want 2", got)
		}
		envelope(w, "0000", map[string]any{
			"accessToken": "tok-123",
			"userId":      "uid-abc",
		})
	})
	c, _ := newTestClient(t, mock)
	c.token = "" // force fresh login
	c.userID = ""

	if err := c.Login(); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if c.token != "tok-123" {
		t.Errorf("token = %q, want tok-123", c.token)
	}
	if c.userID != "uid-abc" {
		t.Errorf("userID = %q, want uid-abc", c.userID)
	}
	// cache file should exist
	if _, err := os.Stat(c.tokenPath()); err != nil {
		t.Errorf("token cache not written: %v", err)
	}
}

func TestLogin_ErrorReturnsAPIError(t *testing.T) {
	mock := newMock(t).on("/account/login", func(w http.ResponseWriter, r *http.Request) {
		envelope(w, "4001", nil)
	})
	c, _ := newTestClient(t, mock)
	c.token = ""
	err := c.Login()
	if err == nil {
		t.Fatal("want error")
	}
	if !strings.Contains(err.Error(), "4001") {
		t.Errorf("err = %v, want to contain 4001", err)
	}
}

func TestDo_SetsAccessTokenAndYfheader(t *testing.T) {
	var gotAccess, gotAccessLower, gotYf string
	mock := newMock(t).on("/analyse/query", func(w http.ResponseWriter, r *http.Request) {
		gotAccess = r.Header.Get("accessToken")
		gotAccessLower = r.Header.Get("accesstoken")
		gotYf = r.Header.Get("yfheader")
		envelope(w, "0000", map[string]any{})
	})
	c, _ := newTestClient(t, mock)
	if _, err := c.QueryAnalytics(); err != nil {
		t.Fatalf("QueryAnalytics: %v", err)
	}
	if gotAccess != "test-token" || gotAccessLower != "test-token" {
		t.Errorf("accessToken headers = %q/%q", gotAccess, gotAccessLower)
	}
	if gotYf != `{"userId":"u42"}` {
		t.Errorf("yfheader = %q", gotYf)
	}
}

func TestDoWithRetry_RetriesOnAPIError(t *testing.T) {
	calls := 0
	mock := newMock(t)
	mock.on("/account/login", func(w http.ResponseWriter, r *http.Request) {
		envelope(w, "0000", map[string]any{"accessToken": "new-tok", "userId": "u42"})
	})
	mock.on("/analyse/query", func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			envelope(w, "1001", nil) // force retry
			return
		}
		envelope(w, "0000", map[string]any{})
	})
	c, _ := newTestClient(t, mock)
	if _, err := c.QueryAnalytics(); err != nil {
		t.Fatalf("QueryAnalytics: %v", err)
	}
	if calls != 2 {
		t.Errorf("analyse calls = %d, want 2", calls)
	}
	if c.token != "new-tok" {
		t.Errorf("token after retry = %q, want new-tok", c.token)
	}
}

func TestDoWithRetry_GivesUpOnHTTPError(t *testing.T) {
	calls := 0
	mock := newMock(t).on("/analyse/query", func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "nope", http.StatusInternalServerError)
	})
	c, _ := newTestClient(t, mock)
	_, err := c.QueryAnalytics()
	if err == nil {
		t.Fatal("want error")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (no retry on 5xx)", calls)
	}
}

func TestTokenCache_RoundTrip(t *testing.T) {
	c, _ := newTestClient(t, newMock(t))
	c.token = "cached-tok"
	c.userID = "uid"
	if err := c.saveToken(); err != nil {
		t.Fatalf("saveToken: %v", err)
	}
	// New client sharing the cache dir should load it
	c2, err := New(Config{
		Region:   "test",
		CacheDir: c.cacheDir,
		BaseURL:  "http://ignored",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tok, uid := c2.loadToken()
	if tok != "cached-tok" || uid != "uid" {
		t.Errorf("loadToken = %q/%q", tok, uid)
	}
}

func TestEnsureAuth_UsesCachedToken(t *testing.T) {
	mock := newMock(t)
	// No /account/login registered — if ensureAuth tries to login, the mock
	// will call t.Errorf from ServeHTTP.
	c, _ := newTestClient(t, mock)
	c.token = ""
	c.userID = ""
	// Pre-seed the cache
	c.token = "cached"
	c.userID = "u42"
	if err := c.saveToken(); err != nil {
		t.Fatalf("saveToken: %v", err)
	}
	c.token = ""
	c.userID = ""
	if err := c.EnsureAuth(); err != nil {
		t.Fatalf("EnsureAuth: %v", err)
	}
	if c.token != "cached" || c.userID != "u42" {
		t.Errorf("after EnsureAuth: %q/%q", c.token, c.userID)
	}
}

// Belt-and-suspenders: make sure a response body larger than the default
// buffer is read correctly.
func TestDo_LargeBody(t *testing.T) {
	blob := strings.Repeat("x", 64*1024)
	mock := newMock(t).on("/analyse/query", func(w http.ResponseWriter, r *http.Request) {
		envelope(w, "0000", map[string]any{"sportStatistic": []map[string]any{{"name": blob}}})
	})
	c, _ := newTestClient(t, mock)
	if _, err := c.QueryAnalytics(); err != nil {
		t.Fatalf("QueryAnalytics: %v", err)
	}
}

// sanity: io.ReadAll paths don't leak the response body
func TestDo_ClosesBody(t *testing.T) {
	var closed bool
	mock := newMock(t).on("/analyse/query", func(w http.ResponseWriter, r *http.Request) {
		envelope(w, "0000", map[string]any{})
	})
	c, ts := newTestClient(t, mock)
	orig := c.http.Transport
	c.http.Transport = &closeTrackingTransport{inner: http.DefaultTransport, closedFlag: &closed}
	t.Cleanup(func() { c.http.Transport = orig })
	_ = ts // keep server var alive
	if _, err := c.QueryAnalytics(); err != nil {
		t.Fatalf("QueryAnalytics: %v", err)
	}
	if !closed {
		t.Errorf("response body not closed")
	}
}

type closeTrackingTransport struct {
	inner      http.RoundTripper
	closedFlag *bool
}

func (c *closeTrackingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := c.inner.RoundTrip(r)
	if err != nil {
		return resp, err
	}
	resp.Body = &trackClose{ReadCloser: resp.Body, flag: c.closedFlag}
	return resp, nil
}

type trackClose struct {
	io.ReadCloser
	flag *bool
}

func (t *trackClose) Close() error {
	*t.flag = true
	return t.ReadCloser.Close()
}
