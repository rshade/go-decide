package jevclient

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

// WithResponseCache stores each successful response in dir and answers a
// repeated request from there without a network call. See the package doc.
func WithResponseCache(dir string) Option {
	return func(c *config) { c.cacheDir = dir }
}

// cacheEntry is what a cache file holds: never a header from the request.
type cacheEntry struct {
	Status      int             `json:"status"`
	ContentType string          `json:"content_type"`
	Body        json.RawMessage `json:"body"`
}

type cachingTransport struct {
	dir  string
	next http.RoundTripper
}

// newCachingClient returns an HTTP client that serves from and fills the cache
// in dir. It refuses redirects as the default jev client does. The directory is
// created on the first write, so a client that fails construction or never
// stores a response leaves nothing behind.
func newCachingClient(dir string) *http.Client {
	return &http.Client{
		Transport: cachingTransport{dir: dir, next: http.DefaultTransport},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (t cachingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, err := readBody(req)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(t.dir, cacheKey(req, body)+".json")

	if resp, ok := t.load(req, path); ok {
		return resp, nil
	}

	resp, err := t.next.RoundTrip(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return resp, err
	}
	respBody, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(respBody))
	if answersEveryQuestion(body, respBody) {
		t.store(path, cacheEntry{Status: resp.StatusCode, ContentType: resp.Header.Get("Content-Type"), Body: respBody})
	}
	return resp, nil
}

// answersEveryQuestion reports whether a 200 body answers every question the
// request asked. A body that misses one breaks the API contract and would fail
// the same way on every rerun, so it is not stored. Checking each answer's
// content stays with the client.
func answersEveryQuestion(reqBody, respBody []byte) bool {
	var req struct {
		Questions map[string]json.RawMessage `json:"questions"`
	}
	var resp struct {
		Answers map[string]json.RawMessage `json:"answers"`
	}
	if json.Unmarshal(reqBody, &req) != nil || json.Unmarshal(respBody, &resp) != nil || len(req.Questions) == 0 {
		return false
	}
	for name := range req.Questions {
		if _, ok := resp.Answers[name]; !ok {
			return false
		}
	}
	return true
}

// readBody reads the request body and puts a fresh reader back, so the request
// that goes on to the server is unchanged.
func readBody(req *http.Request) ([]byte, error) {
	if req.Body == nil || req.Body == http.NoBody {
		return nil, nil
	}
	body, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	return body, nil
}

// cacheKey hashes what identifies a call. Headers are left out, so the bearer
// token never reaches the key.
func cacheKey(req *http.Request, body []byte) string {
	h := sha256.New()
	for _, part := range []string{req.Method, req.URL.Scheme, req.URL.Host, req.URL.Path} {
		_, _ = io.WriteString(h, strconv.Itoa(len(part))+":"+part)
	}
	_, _ = h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

// load answers from the cache. An entry that cannot be read or parsed is a
// miss, and the response that follows replaces it.
func (t cachingTransport) load(req *http.Request, path string) (*http.Response, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var e cacheEntry
	if json.Unmarshal(raw, &e) != nil || e.Status != http.StatusOK || len(e.Body) == 0 {
		return nil, false
	}
	return &http.Response{
		Status:        "200 OK",
		StatusCode:    http.StatusOK,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{"Content-Type": []string{e.ContentType}},
		Body:          io.NopCloser(bytes.NewReader(e.Body)),
		ContentLength: int64(len(e.Body)),
		Request:       req,
	}, true
}

// store writes the entry under a temporary name and renames it, so a crash
// leaves no truncated entry. A write that fails only costs a later request.
func (t cachingTransport) store(path string, e cacheEntry) {
	raw, err := json.Marshal(e)
	if err != nil {
		return
	}
	if os.MkdirAll(t.dir, 0o700) != nil {
		return
	}
	tmp, err := os.CreateTemp(t.dir, ".entry-*")
	if err != nil {
		return
	}
	_, werr := tmp.Write(raw)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		_ = os.Remove(tmp.Name())
		return
	}
	if os.Rename(tmp.Name(), path) != nil {
		_ = os.Remove(tmp.Name())
	}
}
