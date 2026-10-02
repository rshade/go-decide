package jevclient

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cachedClient(t *testing.T, baseURL, dir string) func() error {
	t.Helper()
	setEnv(t, "cache-secret-key", baseURL)
	c, err := NewClient(fastBackoff, WithResponseCache(dir))
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}
	return func() error {
		_, err := c.SystemOne(context.Background(), safeRequest())
		return err
	}
}

func cacheFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading cache dir: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, filepath.Join(dir, e.Name()))
	}
	return names
}

func TestCacheHitSendsNothing(t *testing.T) {
	srv, hits := newServer(t, okHandler)
	call := cachedClient(t, srv.URL, t.TempDir())

	for i := range 2 {
		if err := call(); err != nil {
			t.Fatalf("call %d error: %v", i+1, err)
		}
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("server received %d requests, want 1", got)
	}
}

func TestCacheStoresOnlySuccess(t *testing.T) {
	url, hits := statusServer(t, http.StatusInternalServerError, http.StatusOK)
	dir := t.TempDir()
	call := cachedClient(t, url, dir)

	if err := call(); err == nil {
		t.Fatal("first call succeeded on a 500, want error")
	}
	if files := cacheFiles(t, dir); len(files) != 0 {
		t.Fatalf("a 500 was cached: %v", files)
	}
	for i := range 2 {
		if err := call(); err != nil {
			t.Fatalf("call %d after the 500 error: %v", i+2, err)
		}
	}
	if got := hits.Load(); got != 2 {
		t.Fatalf("server received %d requests, want 2: the 500, then one success served from cache after", got)
	}
}

func TestCacheSkipsRetriedStatusesAndKeepsTheRetriedSuccess(t *testing.T) {
	url, hits := statusServer(t, http.StatusTooManyRequests, http.StatusOK)
	dir := t.TempDir()
	call := cachedClient(t, url, dir)

	if err := call(); err != nil {
		t.Fatalf("call error: %v", err)
	}
	if err := call(); err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if got := hits.Load(); got != 2 {
		t.Fatalf("server received %d requests, want 2: one 429 retried once, then a cache hit", got)
	}
	if files := cacheFiles(t, dir); len(files) != 1 {
		t.Fatalf("cache holds %d entries, want 1", len(files))
	}
}

func TestCacheDoesNotStoreAResponseMissingAnAnswer(t *testing.T) {
	for name, body := range map[string]string{
		"no answers":                 `{"model":"m","usage":{"input_tokens":1,"output_tokens":1}}`,
		"answer to another question": `{"model":"m","answers":{"other":{"type":"noul","noul":0.5}},"usage":{"input_tokens":1,"output_tokens":1}}`,
	} {
		t.Run(name, func(t *testing.T) {
			srv, hits := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			})
			dir := t.TempDir()
			call := cachedClient(t, srv.URL, dir)

			for range 2 {
				if err := call(); err == nil {
					t.Fatal("call succeeded without the requested answer, want error")
				}
			}
			if got := hits.Load(); got != 2 || len(cacheFiles(t, dir)) != 0 {
				t.Fatalf("%d requests and %d cache entries, want 2 and 0", got, len(cacheFiles(t, dir)))
			}
		})
	}
}

func TestCacheReplacesACorruptEntry(t *testing.T) {
	srv, hits := newServer(t, okHandler)
	dir := t.TempDir()
	call := cachedClient(t, srv.URL, dir)
	if err := call(); err != nil {
		t.Fatalf("call error: %v", err)
	}
	files := cacheFiles(t, dir)
	if len(files) != 1 {
		t.Fatalf("cache holds %d entries, want 1", len(files))
	}
	if err := os.WriteFile(files[0], []byte(`{"status":200,"bo`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := call(); err != nil {
		t.Fatalf("call after truncation error: %v", err)
	}
	if got := hits.Load(); got != 2 {
		t.Fatalf("server received %d requests, want 2: the truncated entry is a miss", got)
	}
	raw, err := os.ReadFile(files[0])
	if err != nil || !strings.Contains(string(raw), "jev-latest") {
		t.Fatalf("entry was not replaced: %s, %v", raw, err)
	}
}

func TestCacheKeepsTheTokenOutAndIsPrivate(t *testing.T) {
	srv, _ := newServer(t, okHandler)
	dir := filepath.Join(t.TempDir(), "cache")
	call := cachedClient(t, srv.URL, dir)
	if err := call(); err != nil {
		t.Fatalf("call error: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("cache dir mode = %v, %v; want 0700", info.Mode().Perm(), err)
	}
	for _, f := range cacheFiles(t, dir) {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "cache-secret-key") || strings.Contains(f, "cache-secret-key") {
			t.Fatalf("%s contains the token", f)
		}
		fi, _ := os.Stat(f)
		if fi.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode = %v, want 0600", f, fi.Mode().Perm())
		}
	}
}

func TestCacheStillRefusesRedirects(t *testing.T) {
	target, targetHits := newServer(t, okHandler)
	origin, _ := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusFound)
	})
	call := cachedClient(t, origin.URL, t.TempDir())

	if err := call(); err == nil {
		t.Fatal("call succeeded across a redirect, want error")
	}
	if got := targetHits.Load(); got != 0 {
		t.Fatalf("redirect target received %d requests, want 0", got)
	}
}

func TestNoCacheOptionWritesNothing(t *testing.T) {
	srv, _ := newServer(t, okHandler)
	wd := t.TempDir()
	t.Chdir(wd)
	setEnv(t, "test-key", srv.URL)
	c, err := NewClient(fastBackoff)
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}
	if _, err := c.SystemOne(context.Background(), safeRequest()); err != nil {
		t.Fatalf("call error: %v", err)
	}
	if files := cacheFiles(t, wd); len(files) != 0 {
		t.Fatalf("a client without the cache wrote %v", files)
	}
}

func TestCacheDirIsNotCreatedWhenConstructionFails(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	setEnv(t, "", "")
	if _, err := NewClient(WithResponseCache(dir)); err == nil {
		t.Fatal("NewClient() succeeded without a key, want error")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("cache dir exists after a failed construction: %v", err)
	}
}
