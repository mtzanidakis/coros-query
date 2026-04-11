package coros

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

var regionHosts = map[string]string{
	"eu": "https://teameuapi.coros.com",
	"us": "https://teamapi.coros.com",
	"cn": "https://teamcnapi.coros.com",
}

// Config groups all Client knobs. BaseURL, CacheDir and HTTP are overrides
// intended for tests; production callers set Region/Email/Password/UserID only.
type Config struct {
	Region   string
	Email    string
	Password string
	UserID   string

	BaseURL  string       // override (e.g. httptest server); defaults to region host
	CacheDir string       // override token-cache directory; defaults to os.UserCacheDir()/coros-query
	HTTP     *http.Client // override; defaults to &http.Client{Timeout: 30s}
}

type Client struct {
	http     *http.Client
	baseURL  string
	region   string
	cacheDir string
	email    string
	password string
	token    string
	userID   string
}

// New constructs a Client from Config. BaseURL and CacheDir are derived from
// Region when empty.
func New(cfg Config) (*Client, error) {
	base := cfg.BaseURL
	if base == "" {
		b, ok := regionHosts[cfg.Region]
		if !ok {
			return nil, fmt.Errorf("unknown region %q (want eu, us, cn)", cfg.Region)
		}
		base = b
	}
	httpClient := cfg.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		http:     httpClient,
		baseURL:  base,
		region:   cfg.Region,
		cacheDir: cfg.CacheDir,
		email:    cfg.Email,
		password: cfg.Password,
		userID:   cfg.UserID,
	}, nil
}

func (c *Client) UserID() string { return c.userID }

type APIError struct {
	Result  string
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("coros api error result=%s: %s", e.Result, e.Message)
}

type baseResp struct {
	APICode string          `json:"apiCode"`
	Message string          `json:"message"`
	Result  string          `json:"result"`
	Data    json.RawMessage `json:"data"`
}

// do executes req, validates the Coros envelope, and decodes Data into out.
// It uses both accessToken and accesstoken headers (Coros is inconsistent across endpoints).
func (c *Client) do(req *http.Request, out any) error {
	if c.token != "" {
		req.Header.Set("accessToken", c.token)
		req.Header.Set("accesstoken", c.token)
	}
	if c.userID != "" {
		req.Header.Set("yfheader", fmt.Sprintf(`{"userId":"%s"}`, c.userID))
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("http %d: %s", resp.StatusCode, body)
	}
	var br baseResp
	if err := json.Unmarshal(body, &br); err != nil {
		return fmt.Errorf("decode envelope: %w; body=%s", err, body)
	}
	if br.Result != "0000" {
		return &APIError{Result: br.Result, Message: br.Message}
	}
	if out != nil && len(br.Data) > 0 {
		if err := json.Unmarshal(br.Data, out); err != nil {
			return fmt.Errorf("decode data: %w", err)
		}
	}
	return nil
}

// EnsureAuth is the exported variant of ensureAuth, for callers that want to
// force authentication before fanning out parallel requests.
func (c *Client) EnsureAuth() error { return c.ensureAuth() }

// ensureAuth makes sure we have a token (from cache or a fresh login).
// It also restores a cached userID when the client was constructed without one.
func (c *Client) ensureAuth() error {
	if c.token != "" {
		return nil
	}
	if tok, uid := c.loadToken(); tok != "" {
		c.token = tok
		if c.userID == "" {
			c.userID = uid
		}
		return nil
	}
	return c.Login()
}

// doWithRetry runs the request; on APIError it tries a fresh login once and retries.
func (c *Client) doWithRetry(build func() (*http.Request, error), out any) error {
	if err := c.ensureAuth(); err != nil {
		return err
	}
	req, err := build()
	if err != nil {
		return err
	}
	err = c.do(req, out)
	if err == nil {
		return nil
	}
	if _, ok := err.(*APIError); !ok {
		return err
	}
	if err := c.Login(); err != nil {
		return fmt.Errorf("re-login: %w", err)
	}
	req, err = build()
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// loginData captures fields we care about from the login response. Coros
// returns more fields; we accept a superset. UserID may be numeric or string
// depending on the endpoint, so we capture it via a flexible decode.
type loginData struct {
	AccessToken string          `json:"accessToken"`
	UserID      json.RawMessage `json:"userId"`
}

func (c *Client) Login() error {
	body, _ := json.Marshal(map[string]any{
		"account":     c.email,
		"accountType": 2,
		"pwd":         md5Hex(c.password),
	})
	req, err := http.NewRequest("POST", c.baseURL+"/account/login", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	var out loginData
	prev := c.token
	c.token = ""
	if err := c.do(req, &out); err != nil {
		c.token = prev
		return fmt.Errorf("login: %w", err)
	}
	c.token = out.AccessToken
	if c.userID == "" && len(out.UserID) > 0 {
		// Strip quotes if string-encoded; otherwise use raw number.
		s := string(out.UserID)
		if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
			s = s[1 : len(s)-1]
		}
		c.userID = s
	}
	_ = c.saveToken()
	return nil
}

type tokenFile struct {
	Token   string    `json:"accessToken"`
	UserID  string    `json:"userId,omitempty"`
	SavedAt time.Time `json:"saved_at"`
}

func (c *Client) tokenPath() string {
	dir := c.cacheDir
	if dir == "" {
		d, err := os.UserCacheDir()
		if err != nil {
			d = os.TempDir()
		}
		dir = filepath.Join(d, "coros-query")
	}
	region := c.region
	if region == "" {
		region = "default"
	}
	return filepath.Join(dir, fmt.Sprintf("token-%s.json", region))
}

func (c *Client) loadToken() (string, string) {
	data, err := os.ReadFile(c.tokenPath())
	if err != nil {
		return "", ""
	}
	var tf tokenFile
	if err := json.Unmarshal(data, &tf); err != nil {
		return "", ""
	}
	return tf.Token, tf.UserID
}

func (c *Client) saveToken() error {
	p := c.tokenPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, _ := json.Marshal(tokenFile{Token: c.token, UserID: c.userID, SavedAt: time.Now()})
	return os.WriteFile(p, data, 0o600)
}
