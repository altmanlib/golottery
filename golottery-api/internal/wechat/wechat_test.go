package wechat

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"golottery/api/internal/redisx"
	"golottery/api/internal/wechat/wechattest"
)

func newClient(fake *wechattest.Server, rdb *redis.Client) *Client {
	return New(Config{AppID: "wx123", AppSecret: "secret", BaseURL: fake.URL, EnvVersion: "develop", Redis: rdb})
}

func TestInstancesShareOneCachedToken(t *testing.T) {
	fake := wechattest.New()
	defer fake.Close()
	rdb := redisx.OpenTest(t)
	ctx := context.Background()

	a, err := newClient(fake, rdb).AccessToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b, err := newClient(fake, rdb).AccessToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if a != b || fake.TokenCalls() != 1 {
		t.Fatalf("tokens %q %q after %d calls, want one shared token", a, b, fake.TokenCalls())
	}
	ttl := rdb.TTL(ctx, TokenKey).Val()
	if ttl <= 0 || ttl > 6900*time.Second {
		t.Fatalf("cache ttl = %v, want expires_in minus 300s", ttl)
	}
}

func TestExpiredCacheIsRefetched(t *testing.T) {
	fake := wechattest.New()
	defer fake.Close()
	fake.ExpiresIn = 301
	rdb := redisx.OpenTest(t)
	client := newClient(fake, rdb)
	ctx := context.Background()

	first, _ := client.AccessToken(ctx)
	time.Sleep(1100 * time.Millisecond)
	second, _ := client.AccessToken(ctx)
	if first == second || fake.TokenCalls() != 2 {
		t.Fatalf("tokens %q %q after %d calls, want a refetch after expiry", first, second, fake.TokenCalls())
	}
}

func TestRejectedTokenIsRefetchedOnce(t *testing.T) {
	fake := wechattest.New()
	defer fake.Close()
	rdb := redisx.OpenTest(t)
	client := newClient(fake, rdb)
	ctx := context.Background()

	fake.RejectTokens = 1
	img, err := client.UnlimitedQRCode(ctx, "V1StGXR8_Z5jdHi6B-myT", "pages/index/index")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(img)); err != nil {
		t.Fatalf("qrcode is not a PNG: %v", err)
	}
	calls := fake.QRCalls()
	if fake.TokenCalls() != 2 || len(calls) != 2 || calls[0].Token == calls[1].Token {
		t.Fatalf("token calls %d, qr calls %+v", fake.TokenCalls(), calls)
	}

	fake.RejectTokens = 5
	_, err = client.UnlimitedQRCode(ctx, "x", "pages/index/index")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != 40001 {
		t.Fatalf("error = %v, want errcode 40001", err)
	}
	if got := len(fake.QRCalls()) - len(calls); got != 2 {
		t.Fatalf("retried %d times, want exactly one retry", got-1)
	}
}

func TestQRCodeRequest(t *testing.T) {
	fake := wechattest.New()
	defer fake.Close()
	client := newClient(fake, redisx.OpenTest(t))
	if _, err := client.UnlimitedQRCode(context.Background(), "V1StGXR8_Z5jdHi6B-myT", "pages/index/index"); err != nil {
		t.Fatal(err)
	}
	call := fake.QRCalls()[0]
	if call.Scene != "V1StGXR8_Z5jdHi6B-myT" || call.Page != "pages/index/index" || call.EnvVersion != "develop" || call.CheckPath {
		t.Fatalf("request = %+v", call)
	}

	fake.QRErrCode = 41030
	_, err := client.UnlimitedQRCode(context.Background(), "x", "pages/missing")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != 41030 {
		t.Fatalf("error = %v, want errcode 41030", err)
	}
}

func TestWorksWithoutRedis(t *testing.T) {
	fake := wechattest.New()
	defer fake.Close()
	client := newClient(fake, redisx.Unreachable())
	if _, err := client.UnlimitedQRCode(context.Background(), "x", "pages/index/index"); err != nil {
		t.Fatalf("qrcode with Redis down: %v", err)
	}
	if _, err := New(Config{}).AccessToken(context.Background()); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("unconfigured error = %v", err)
	}
}
