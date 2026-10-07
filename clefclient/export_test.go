package clefclient_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/kataras/jev"

	"github.com/rshade/go-decide/clefclient"
)

const account = "0123456789abcdef0123456789abcdef"

func fixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/response.json")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestBaseURL(t *testing.T) {
	got, err := clefclient.BaseURL(strings.ToUpper(account))
	want := "https://api.cloudflare.com/client/v4/accounts/" + account + "/ai/run/@cf/cloudflare"
	if err != nil || got != want {
		t.Errorf("BaseURL() = %q, %v; want %q", got, err, want)
	}
	for _, bad := range []string{"", "short", "../" + account[3:], account + "0"} {
		if _, err := clefclient.BaseURL(bad); err == nil {
			t.Errorf("BaseURL(%q) accepted a bad account", bad)
		}
	}
}

func TestTransportLetsAPlainHTTPClientReachClef(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write(fixture(t))
	}))
	defer srv.Close()

	client := &http.Client{
		Transport:     clefclient.NewTransport(nil),
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	base := srv.URL + "/client/v4/accounts/" + account + "/ai/run/@cf/cloudflare"
	req, _ := http.NewRequest(http.MethodPost, base+"/v1/systemone",
		strings.NewReader(`{"model":"clef","state":"x","questions":{"urgent":{"type":"noul","instructions":"y"}}}`))
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)

	if want := "/client/v4/accounts/" + account + "/ai/run/@cf/cloudflare/clef"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
	if gotAuth != "Bearer tok" || gotBody["model"] != "clef" {
		t.Errorf("auth %q body %v", gotAuth, gotBody)
	}
	var flat struct {
		Model   string                     `json:"model"`
		Answers map[string]json.RawMessage `json:"answers"`
	}
	if err := json.Unmarshal(body, &flat); err != nil || flat.Model != "clef" || len(flat.Answers) != 3 {
		t.Errorf("body %s is not a flat System One response: %v", body, err)
	}
}

func TestTransportRefusesOtherPaths(t *testing.T) {
	client := &http.Client{Transport: clefclient.NewTransport(nil)}
	for _, tt := range []struct{ method, url string }{
		{http.MethodGet, "https://127.0.0.1:1/v1/models"},
		{http.MethodGet, "https://127.0.0.1:1/v1/systemone"},
		{http.MethodPost, "https://127.0.0.1:1/anything"},
	} {
		req, _ := http.NewRequest(tt.method, tt.url, nil)
		if resp, err := client.Do(req); err == nil {
			_ = resp.Body.Close()
			t.Errorf("%s %s was sent", tt.method, tt.url)
		}
	}
}

func TestRecordedResponseAnswersAllThreeQuestionTypes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(fixture(t)) }))
	defer srv.Close()
	t.Setenv("CLOUDFLARE_AUTH_TOKEN", "tok")
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", account)
	t.Setenv("CLOUDFLARE_BASE_URL", srv.URL)
	t.Setenv("CLOUDFLARE_LOG_LEVEL", "")

	c, err := clefclient.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.SystemOne(context.Background(), jev.Request{
		State: "Checkout has been failing for every customer for the last hour.",
		Questions: jev.Questions{
			"urgent":   jev.Noul{Instructions: "Is this support request urgent?"},
			"team":     jev.Choice{Instructions: "Which team?", Criteria: map[string]string{"billing": "b", "technical": "t", "sales": "s"}},
			"severity": jev.Score{Instructions: "How severe?", Criteria: []string{"No impact", "Minor", "Major", "Critical"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n, ok := resp.Noul("urgent"); !ok || n.Noul != 0.9905 {
		t.Errorf("noul = %+v, %v", n, ok)
	}
	if ch, ok := resp.Choice("team"); !ok || ch.Choice != "technical" || ch.Confidence != 0.5286 {
		t.Errorf("choice = %+v, %v", ch, ok)
	}
	if sc, ok := resp.Score("severity"); !ok || sc.Score != 2.9571 || sc.Confidence != 0.9211 {
		t.Errorf("score = %+v, %v", sc, ok)
	}
}
