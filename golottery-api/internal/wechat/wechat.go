// Package wechat calls the WeChat mini program server API: the shared access token
// and unlimited mini program codes. Phase 6 adds code-to-openid exchange.
package wechat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/redis/go-redis/v9"

	"golottery/api/internal/redisx"
)

// DefaultBaseURL is the WeChat API host.
const DefaultBaseURL = "https://api.weixin.qq.com"

// TokenKey caches the access token shared by every API instance.
const TokenKey = redisx.KeyPrefix + "wechat:access_token"

// tokenEarlyExpiry refreshes the cache five minutes before WeChat does.
const tokenEarlyExpiry = 300 * time.Second

// ErrNotConfigured means no AppID or AppSecret is set.
var ErrNotConfigured = errors.New("wechat: app id or secret not configured")

// APIError is an errcode answer from WeChat.
type APIError struct {
	Op      string
	Code    int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("wechat: %s: errcode %d: %s", e.Op, e.Code, e.Message)
}

// tokenInvalid lists errcodes that mean the access token is no longer valid.
func tokenInvalid(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.Code {
	case 40001, 40014, 42001:
		return true
	}
	return false
}

// Config configures a Client.
type Config struct {
	AppID      string
	AppSecret  string
	BaseURL    string
	EnvVersion string // release, trial or develop
	Redis      *redis.Client
	HTTP       *http.Client
	Logger     *slog.Logger
}

// Client is safe for concurrent use.
type Client struct {
	cfg Config
}

// New builds a Client; missing optional fields get defaults.
func New(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if cfg.EnvVersion == "" {
		cfg.EnvVersion = "release"
	}
	if cfg.HTTP == nil {
		cfg.HTTP = &http.Client{Timeout: 10 * time.Second}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Client{cfg: cfg}
}

// Configured reports whether AppID and AppSecret are set.
func (c *Client) Configured() bool {
	return c.cfg.AppID != "" && c.cfg.AppSecret != ""
}

// AccessToken returns the shared stable access token, from Redis when cached.
// A Redis failure only skips the cache.
func (c *Client) AccessToken(ctx context.Context) (string, error) {
	if !c.Configured() {
		return "", ErrNotConfigured
	}
	if c.cfg.Redis != nil {
		token, err := c.cfg.Redis.Get(ctx, TokenKey).Result()
		if err == nil && token != "" {
			return token, nil
		}
		if err != nil && !errors.Is(err, redis.Nil) {
			c.cfg.Logger.Warn("wechat token cache read failed", "error", err)
		}
	}
	token, expiresIn, err := c.fetchStableToken(ctx)
	if err != nil {
		return "", err
	}
	if c.cfg.Redis != nil {
		ttl := time.Duration(expiresIn)*time.Second - tokenEarlyExpiry
		if ttl > 0 {
			if err := c.cfg.Redis.Set(ctx, TokenKey, token, ttl).Err(); err != nil {
				c.cfg.Logger.Warn("wechat token cache write failed", "error", err)
			}
		}
	}
	return token, nil
}

// dropToken forgets the cached token after WeChat rejected it.
func (c *Client) dropToken(ctx context.Context) {
	if c.cfg.Redis == nil {
		return
	}
	if err := c.cfg.Redis.Del(ctx, TokenKey).Err(); err != nil {
		c.cfg.Logger.Warn("wechat token cache delete failed", "error", err)
	}
}

// fetchStableToken uses the normal (non-forced) stable_token mode: concurrent calls
// from several instances return the same token and never invalidate each other.
func (c *Client) fetchStableToken(ctx context.Context) (string, int, error) {
	body := map[string]any{
		"grant_type":    "client_credential",
		"appid":         c.cfg.AppID,
		"secret":        c.cfg.AppSecret,
		"force_refresh": false,
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	raw, _, err := c.post(ctx, "/cgi-bin/stable_token", nil, body)
	if err != nil {
		return "", 0, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", 0, fmt.Errorf("wechat: stable_token: decode: %w", err)
	}
	if out.ErrCode != 0 || out.AccessToken == "" {
		return "", 0, &APIError{Op: "stable_token", Code: out.ErrCode, Message: out.ErrMsg}
	}
	return out.AccessToken, out.ExpiresIn, nil
}

// UnlimitedQRCode returns a PNG mini program code that opens page with scene.
// A rejected token is dropped and fetched again once.
func (c *Client) UnlimitedQRCode(ctx context.Context, scene, page string) ([]byte, error) {
	img, err := c.unlimitedQRCode(ctx, scene, page)
	if tokenInvalid(err) {
		c.dropToken(ctx)
		img, err = c.unlimitedQRCode(ctx, scene, page)
	}
	return img, err
}

func (c *Client) unlimitedQRCode(ctx context.Context, scene, page string) ([]byte, error) {
	token, err := c.AccessToken(ctx)
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"scene":       scene,
		"page":        page,
		"env_version": c.cfg.EnvVersion,
		// Unreleased versions cannot pass the published-page check.
		"check_path": c.cfg.EnvVersion == "release",
	}
	raw, contentType, err := c.post(ctx, "/wxa/getwxacodeunlimit", url.Values{"access_token": {token}}, body)
	if err != nil {
		return nil, err
	}
	switch http.DetectContentType(raw) {
	case "image/png":
		return raw, nil
	case "image/jpeg":
		return jpegToPNG(raw)
	}
	var out struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.ErrCode == 0 {
		return nil, fmt.Errorf("wechat: getwxacodeunlimit: unexpected %s response", contentType)
	}
	return nil, &APIError{Op: "getwxacodeunlimit", Code: out.ErrCode, Message: out.ErrMsg}
}

func jpegToPNG(raw []byte) ([]byte, error) {
	img, err := jpeg.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("wechat: decode qrcode: %w", err)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, fmt.Errorf("wechat: encode qrcode: %w", err)
	}
	return out.Bytes(), nil
}

func (c *Client) post(ctx context.Context, path string, query url.Values, body any) ([]byte, string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, "", err
	}
	target := c.cfg.BaseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.cfg.HTTP.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("wechat: %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, "", fmt.Errorf("wechat: %s: read: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("wechat: %s: http %d", path, resp.StatusCode)
	}
	return raw, resp.Header.Get("Content-Type"), nil
}
