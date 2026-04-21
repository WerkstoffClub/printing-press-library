// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
//
// Tests for the do() auto-retry-on-session-expired branch. Every scenario
// uses an httptest.Server with a scripted response sequence; no live calls.

package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/config"
)

// scriptedResponse is a single canned reply from the test server.
type scriptedResponse struct {
	status int
	body   string
	// contentType lets tests simulate Cloudflare HTML for the HTTP-401
	// fallback path.
	contentType string
}

// scriptedServer plays back scriptedResponse values in order, one per request.
// After the script is exhausted, further requests panic — tests should assert
// the exact call count.
type scriptedServer struct {
	srv       *httptest.Server
	mu        sync.Mutex
	script    []scriptedResponse
	next      int
	callCount int32
	// observedTokens records the authToken form field on every inbound
	// request so tests can verify the retry used the fresh token.
	observedTokens []string
}

func newScriptedServer(t *testing.T, script ...scriptedResponse) *scriptedServer {
	t.Helper()
	s := &scriptedServer{script: script}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		atomic.AddInt32(&s.callCount, 1)
		s.mu.Lock()
		s.observedTokens = append(s.observedTokens, r.PostForm.Get("authToken"))
		if s.next >= len(s.script) {
			s.mu.Unlock()
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"error":"script exhausted"}`))
			return
		}
		reply := s.script[s.next]
		s.next++
		s.mu.Unlock()
		ct := reply.contentType
		if ct == "" {
			ct = "application/json"
		}
		w.Header().Set("Content-Type", ct)
		w.WriteHeader(reply.status)
		_, _ = w.Write([]byte(reply.body))
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *scriptedServer) CallCount() int {
	return int(atomic.LoadInt32(&s.callCount))
}

// newRetryTestClient wires a Client with a pre-populated stale authToken and
// the caller's RefreshAuth / AutoRetryOnExpired settings.
func newRetryTestClient(t *testing.T, serverURL string, refreshAuth func(context.Context) error, autoRetry bool) *Client {
	t.Helper()
	cfg := &config.Config{
		BaseURL:            serverURL,
		ExpensifyAuthToken: "stale-token-abc",
		ExpensifyEmail:     "user@example.com",
	}
	c := New(cfg, 5*time.Second, 0 /* limiter disabled */)
	c.RefreshAuth = refreshAuth
	c.AutoRetryOnExpired = autoRetry
	return c
}

func TestAutoRetry_SuccessAfterRefresh(t *testing.T) {
	srv := newScriptedServer(t,
		scriptedResponse{status: 200, body: `{"jsonCode":407,"message":"session expired"}`},
		scriptedResponse{status: 200, body: `{"jsonCode":200,"data":"ok"}`},
	)

	var refreshCalls int32
	var client *Client
	refresh := func(ctx context.Context) error {
		atomic.AddInt32(&refreshCalls, 1)
		// Simulate what the real closure does: mint and persist a fresh
		// token on the shared config. The next retry attempt reads this.
		client.Config.ExpensifyAuthToken = "fresh-token-xyz"
		return nil
	}
	client = newRetryTestClient(t, srv.srv.URL, refresh, true)

	raw, status, err := client.Post("/Search", map[string]any{"q": "x"})
	if err != nil {
		t.Fatalf("Post: unexpected error: %v", err)
	}
	if status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}
	if !strings.Contains(string(raw), `"data":"ok"`) {
		t.Fatalf("body = %s, want payload with data=ok", string(raw))
	}
	if got := atomic.LoadInt32(&refreshCalls); got != 1 {
		t.Fatalf("RefreshAuth called %d times, want 1", got)
	}
	if srv.CallCount() != 2 {
		t.Fatalf("server saw %d requests, want 2", srv.CallCount())
	}
	// The second request should have carried the fresh token.
	if got := srv.observedTokens[1]; got != "fresh-token-xyz" {
		t.Fatalf("retry authToken = %q, want fresh-token-xyz", got)
	}
}

func TestAutoRetry_RefreshFails(t *testing.T) {
	srv := newScriptedServer(t,
		scriptedResponse{status: 200, body: `{"jsonCode":407,"message":"session expired"}`},
	)
	refreshErr := errors.New("keychain corrupted")
	refresh := func(ctx context.Context) error { return refreshErr }
	client := newRetryTestClient(t, srv.srv.URL, refresh, true)

	_, _, err := client.Post("/Search", map[string]any{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "session expired") {
		t.Fatalf("error missing 'session expired' cause: %v", err)
	}
	if !strings.Contains(msg, "re-auth failed") {
		t.Fatalf("error missing 're-auth failed' marker: %v", err)
	}
	if !strings.Contains(msg, "keychain corrupted") {
		t.Fatalf("error missing refresh reason: %v", err)
	}
	if srv.CallCount() != 1 {
		t.Fatalf("server saw %d requests, want 1 (no retry after refresh failure)", srv.CallCount())
	}
}

func TestAutoRetry_RefreshReturnsHeadlessNotConfigured(t *testing.T) {
	srv := newScriptedServer(t,
		scriptedResponse{status: 200, body: `{"jsonCode":407,"message":"session expired"}`},
	)
	var refreshCalls int32
	refresh := func(ctx context.Context) error {
		atomic.AddInt32(&refreshCalls, 1)
		return ErrHeadlessNotConfigured
	}
	client := newRetryTestClient(t, srv.srv.URL, refresh, true)

	_, _, err := client.Post("/Search", map[string]any{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "auth store-credentials") {
		t.Fatalf("hint missing 'auth store-credentials': %v", err)
	}
	if !strings.Contains(msg, "auth login --headless") {
		t.Fatalf("hint missing 'auth login --headless': %v", err)
	}
	if got := atomic.LoadInt32(&refreshCalls); got != 1 {
		t.Fatalf("RefreshAuth called %d times, want 1", got)
	}
}

func TestAutoRetry_SecondAttemptAlso407(t *testing.T) {
	srv := newScriptedServer(t,
		scriptedResponse{status: 200, body: `{"jsonCode":407,"message":"session expired"}`},
		scriptedResponse{status: 200, body: `{"jsonCode":407,"message":"still expired"}`},
	)
	var refreshCalls int32
	var client *Client
	refresh := func(ctx context.Context) error {
		atomic.AddInt32(&refreshCalls, 1)
		client.Config.ExpensifyAuthToken = "fresh-token"
		return nil
	}
	client = newRetryTestClient(t, srv.srv.URL, refresh, true)

	_, _, err := client.Post("/Search", map[string]any{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "auto-retry exhausted") {
		t.Fatalf("error missing 'auto-retry exhausted': %v", err)
	}
	if got := atomic.LoadInt32(&refreshCalls); got != 1 {
		t.Fatalf("RefreshAuth called %d times, want exactly 1", got)
	}
	if srv.CallCount() != 2 {
		t.Fatalf("server saw %d requests, want exactly 2", srv.CallCount())
	}
}

func TestAutoRetry_Disabled(t *testing.T) {
	srv := newScriptedServer(t,
		scriptedResponse{status: 200, body: `{"jsonCode":407,"message":"session expired"}`},
	)
	var refreshCalls int32
	refresh := func(ctx context.Context) error {
		atomic.AddInt32(&refreshCalls, 1)
		return nil
	}
	// autoRetry=false → even with a non-nil RefreshAuth, the branch is skipped
	client := newRetryTestClient(t, srv.srv.URL, refresh, false)

	raw, _, err := client.Post("/Search", map[string]any{})
	if err != nil {
		t.Fatalf("Post returned err %v; 407 with auto-retry disabled should surface to caller as a 200 body (Search helper does classification)", err)
	}
	// With auto-retry disabled, the raw body (including jsonCode 407) is
	// returned to the caller just as it was pre-change.
	if !strings.Contains(string(raw), `"jsonCode":407`) {
		t.Fatalf("body = %s, want raw 407 envelope", string(raw))
	}
	if got := atomic.LoadInt32(&refreshCalls); got != 0 {
		t.Fatalf("RefreshAuth called %d times, want 0 (auto-retry disabled)", got)
	}
	if srv.CallCount() != 1 {
		t.Fatalf("server saw %d requests, want 1", srv.CallCount())
	}
}

func TestAutoRetry_RefreshAuthNil(t *testing.T) {
	srv := newScriptedServer(t,
		scriptedResponse{status: 200, body: `{"jsonCode":407,"message":"session expired"}`},
	)
	// RefreshAuth is nil; AutoRetryOnExpired toggled on has no effect without
	// a callback.
	client := newRetryTestClient(t, srv.srv.URL, nil, true)

	raw, _, err := client.Post("/Search", map[string]any{})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if !strings.Contains(string(raw), `"jsonCode":407`) {
		t.Fatalf("body = %s, want raw 407 envelope (no retry available)", string(raw))
	}
	if srv.CallCount() != 1 {
		t.Fatalf("server saw %d requests, want 1", srv.CallCount())
	}
}

func TestAutoRetry_ConcurrentRequests(t *testing.T) {
	// 3 requests each hit 407 then 200 on retry. With the mutex + token-change
	// detection, RefreshAuth must fire exactly once total.
	const N = 3
	script := make([]scriptedResponse, 0, 2*N)
	for i := 0; i < N; i++ {
		script = append(script, scriptedResponse{status: 200, body: `{"jsonCode":407}`})
	}
	for i := 0; i < N; i++ {
		script = append(script, scriptedResponse{status: 200, body: fmt.Sprintf(`{"jsonCode":200,"n":%d}`, i)})
	}
	srv := newScriptedServer(t, script...)

	var refreshCalls int32
	var client *Client
	refresh := func(ctx context.Context) error {
		atomic.AddInt32(&refreshCalls, 1)
		// Simulate refresh latency so parallel 407s pile up on the mutex.
		time.Sleep(50 * time.Millisecond)
		client.Config.ExpensifyAuthToken = "fresh-token"
		return nil
	}
	client = newRetryTestClient(t, srv.srv.URL, refresh, true)

	var wg sync.WaitGroup
	errs := make([]error, N)
	// Fire all 3 407s first so they queue on the mutex before any retry
	// lands. We do this by using a gate that releases after 3 requests have
	// arrived; in practice, racing 3 POSTs in parallel is enough because
	// the refresh function sleeps 50ms.
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _, err := client.Post("/Search", map[string]any{"i": idx})
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	// With the scripted server returning 407 on the FIRST 3 requests
	// regardless of order, every caller sees a 407 and attempts to refresh.
	// The mutex + token-change check should collapse this to 1 RefreshAuth.
	got := atomic.LoadInt32(&refreshCalls)
	if got != 1 {
		t.Fatalf("RefreshAuth called %d times, want exactly 1 (mutex must collapse concurrent re-auth)", got)
	}
	for i, e := range errs {
		if e != nil {
			t.Errorf("request %d: err = %v", i, e)
		}
	}
	if srv.CallCount() != 2*N {
		t.Fatalf("server saw %d requests, want %d (one 407 + one 200 per caller)", srv.CallCount(), 2*N)
	}
}

func TestAutoRetry_FallbackHTTP401(t *testing.T) {
	// HTTP 401 with non-JSON body (e.g., Cloudflare HTML) — the body-level
	// jsonCode check misses, but the HTTP-status fallback picks it up when
	// the body isn't parseable JSON.
	srv := newScriptedServer(t,
		scriptedResponse{status: 401, body: `<html><body>Unauthorized</body></html>`, contentType: "text/html"},
		scriptedResponse{status: 200, body: `{"jsonCode":200,"data":"ok"}`},
	)
	var refreshCalls int32
	var client *Client
	refresh := func(ctx context.Context) error {
		atomic.AddInt32(&refreshCalls, 1)
		client.Config.ExpensifyAuthToken = "fresh-token"
		return nil
	}
	client = newRetryTestClient(t, srv.srv.URL, refresh, true)

	raw, _, err := client.Post("/Search", map[string]any{})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if !strings.Contains(string(raw), `"data":"ok"`) {
		t.Fatalf("body = %s, want data=ok", string(raw))
	}
	if got := atomic.LoadInt32(&refreshCalls); got != 1 {
		t.Fatalf("RefreshAuth called %d times, want 1", got)
	}
}

func TestAutoRetry_NotExpiryJSONCode(t *testing.T) {
	// jsonCode 402 isn't session-expired — we must NOT attempt refresh.
	srv := newScriptedServer(t,
		scriptedResponse{status: 200, body: `{"jsonCode":402,"message":"something else"}`},
	)
	var refreshCalls int32
	refresh := func(ctx context.Context) error {
		atomic.AddInt32(&refreshCalls, 1)
		return nil
	}
	client := newRetryTestClient(t, srv.srv.URL, refresh, true)

	raw, _, err := client.Post("/Search", map[string]any{})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if !strings.Contains(string(raw), `"jsonCode":402`) {
		t.Fatalf("body = %s, want raw 402 envelope", string(raw))
	}
	if got := atomic.LoadInt32(&refreshCalls); got != 0 {
		t.Fatalf("RefreshAuth called %d times, want 0", got)
	}
}

func TestAutoRetry_429ThenExpiry(t *testing.T) {
	// Compose the two retry paths: first a rate-limit retry, then a 407
	// refresh, then success. Total 3 requests.
	srv := newScriptedServer(t,
		scriptedResponse{status: 429, body: `{"error":"too many"}`},
		scriptedResponse{status: 200, body: `{"jsonCode":407}`},
		scriptedResponse{status: 200, body: `{"jsonCode":200,"data":"ok"}`},
	)
	var refreshCalls int32
	var client *Client
	refresh := func(ctx context.Context) error {
		atomic.AddInt32(&refreshCalls, 1)
		client.Config.ExpensifyAuthToken = "fresh-token"
		return nil
	}
	client = newRetryTestClient(t, srv.srv.URL, refresh, true)

	raw, _, err := client.Post("/Search", map[string]any{})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if !strings.Contains(string(raw), `"data":"ok"`) {
		t.Fatalf("body = %s, want data=ok", string(raw))
	}
	if srv.CallCount() != 3 {
		t.Fatalf("server saw %d requests, want 3 (429 + 407 + 200)", srv.CallCount())
	}
	if got := atomic.LoadInt32(&refreshCalls); got != 1 {
		t.Fatalf("RefreshAuth called %d times, want 1", got)
	}
}
