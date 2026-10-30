package hearthapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

// Client talks to a running Hearth HTTP API using Cookie session + X-CSRF-Token.
type Client struct {
	base string
	http *http.Client
	mu   sync.Mutex
	csrf string
	user string
	pass string
}

func New(baseURL, username, password string) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = "http://127.0.0.1:8080"
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &Client{
		base: base,
		http: &http.Client{Jar: jar},
		user: username,
		pass: password,
	}, nil
}

func (c *Client) EnsureLogin() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.csrf != "" {
		return nil
	}
	return c.loginLocked()
}

func (c *Client) loginLocked() error {
	if strings.TrimSpace(c.user) == "" || c.pass == "" {
		return fmt.Errorf("set HEARTH_USERNAME and HEARTH_PASSWORD")
	}
	body, _ := json.Marshal(map[string]string{
		"username": c.user,
		"password": c.pass,
	})
	req, err := http.NewRequest(http.MethodPost, c.base+"/api/login", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed: %s: %s", resp.Status, truncate(raw))
	}
	var out struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return err
	}
	if out.CSRF == "" {
		return fmt.Errorf("login: missing csrf")
	}
	c.csrf = out.CSRF
	return nil
}

func (c *Client) do(method, path string, query url.Values, body any, mutate bool) ([]byte, int, error) {
	if err := c.EnsureLogin(); err != nil {
		return nil, 0, err
	}
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		rdr = bytes.NewReader(b)
	}
	u := c.base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequest(method, u, rdr)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if mutate {
		c.mu.Lock()
		csrf := c.csrf
		c.mu.Unlock()
		req.Header.Set("X-CSRF-Token", csrf)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode == http.StatusUnauthorized || (mutate && resp.StatusCode == http.StatusForbidden && bytes.Contains(raw, []byte(`"csrf"`))) {
		c.mu.Lock()
		c.csrf = ""
		loginErr := c.loginLocked()
		c.mu.Unlock()
		if loginErr != nil {
			return raw, resp.StatusCode, loginErr
		}
		return c.do(method, path, query, body, mutate)
	}
	return raw, resp.StatusCode, nil
}

func (c *Client) Get(path string, query url.Values) (json.RawMessage, error) {
	raw, status, err := c.do(http.MethodGet, path, query, nil, false)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("%s %s: %s: %s", http.MethodGet, path, http.StatusText(status), truncate(raw))
	}
	return json.RawMessage(raw), nil
}

func (c *Client) Mutate(method, path string, body any) (json.RawMessage, error) {
	raw, status, err := c.do(method, path, nil, body, true)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("%s %s: %s: %s", method, path, http.StatusText(status), truncate(raw))
	}
	if len(raw) == 0 {
		return json.RawMessage(`{"ok":true}`), nil
	}
	return json.RawMessage(raw), nil
}

func truncate(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		return s[:300] + "..."
	}
	return s
}

func IntQuery(v int64) string {
	if v <= 0 {
		return ""
	}
	return strconv.FormatInt(v, 10)
}
