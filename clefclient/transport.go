package clefclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	systemOneSuffix = "/v1/systemone"
	maxBodyBytes    = 8 << 20
)

// NewTransport returns a RoundTripper that adapts Cloudflare to the System One
// wire format, so any client that posts System One requests can reach clef.
// Give that client the URL from [BaseURL], the bearer token, and the model name
// clef in each request body; nothing else about the request changes. The
// transport swaps the path /v1/systemone for the clef run path and replaces the
// body of a successful response with the envelope's result, so the client sees
// a flat System One response. A 200 that reports failure becomes a 502 that
// keeps the original body, so [Classify] can read the Cloudflare errors. Any
// other path or method is refused. A nil next means [http.DefaultTransport].
//
// The transport does not follow redirects, retry, or hold the token: those stay
// with the caller's client, which must refuse redirects so the token cannot
// reach another host.
func NewTransport(next http.RoundTripper) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	return envelopeTransport{next: next}
}

type envelopeTransport struct {
	next http.RoundTripper
}

func (t envelopeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodPost || !strings.HasSuffix(req.URL.Path, systemOneSuffix) {
		return nil, fmt.Errorf("clefclient: only POST %s is supported, got %s %s", systemOneSuffix, req.Method, req.URL.Path)
	}
	out := req.Clone(req.Context())
	out.URL.Path = strings.TrimSuffix(req.URL.Path, systemOneSuffix) + "/" + modelName
	out.URL.RawPath = ""

	resp, err := t.next.RoundTrip(out)
	if err != nil || resp.StatusCode != http.StatusOK {
		return resp, err
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	var env struct {
		Result  json.RawMessage `json:"result"`
		Success *bool           `json:"success"`
	}
	body := raw
	if json.Unmarshal(raw, &env) != nil || (env.Success != nil && !*env.Success) || len(env.Result) == 0 {
		resp.StatusCode = http.StatusBadGateway
		resp.Status = "502 Bad Gateway"
	} else {
		body = env.Result
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Del("Content-Length")
	return resp, nil
}
