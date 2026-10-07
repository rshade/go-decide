//go:build probe

package gojev_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kataras/jev"
)

// Probe: does Cloudflare's clef answer System One questions as well as Jev?
// Run: go test -tags probe -run TestClef -v -count=1
// Needs CLOUDFLARE_ACCOUNT_ID and CLOUDFLARE_AUTH_TOKEN in the environment or
// in ./.env. Skips otherwise. Jev columns appear only when TYPESAFE_API_KEY is
// set, and come from .probe-cache when the spike already ran.

func dotenv(name string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	f, err := os.Open(".env")
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if k, v, ok := strings.Cut(strings.TrimSpace(sc.Text()), "="); ok && k == name {
			return strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return ""
}

// clefTransport makes a jev.Client talk to Cloudflare: it rewrites
// /v1/systemone to the clef run path and unwraps the {result: ...} envelope
// so jev sees a System One response.
type clefTransport struct {
	account string
	base    http.RoundTripper
}

func (c clefTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.URL.Path = "/client/v4/accounts/" + c.account + "/ai/run/@cf/cloudflare/clef"
	resp, err := c.base.RoundTrip(r)
	if err != nil || resp.StatusCode != http.StatusOK {
		return resp, err
	}
	raw, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	var env struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw, &env); err != nil || len(env.Result) == 0 {
		return nil, fmt.Errorf("clef: unexpected envelope: %s", trunc(string(raw), 200))
	}
	resp.Body = io.NopCloser(bytes.NewReader(env.Result))
	resp.ContentLength = int64(len(env.Result))
	return resp, nil
}

func newClefClient(t *testing.T) *jev.Client {
	t.Helper()
	acct, token := dotenv("CLOUDFLARE_ACCOUNT_ID"), dotenv("CLOUDFLARE_AUTH_TOKEN")
	if acct == "" || token == "" {
		t.Skip("CLOUDFLARE_ACCOUNT_ID or CLOUDFLARE_AUTH_TOKEN not set")
	}
	c, err := jev.New(
		jev.WithAPIKey(token),
		jev.WithBaseURL("https://api.cloudflare.com"),
		jev.WithTimeout(30*time.Second),
		jev.WithHTTPClient(&http.Client{
			Transport:     clefTransport{account: acct, base: http.DefaultTransport},
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}),
	)
	if err != nil {
		t.Fatalf("new clef client: %v", err)
	}
	t.Cleanup(c.CloseIdleConnections)
	return c
}

func clefSpike(t *testing.T, c *jev.Client, ds []spikeDecision, reverse bool) spikeRun {
	t.Helper()
	res, err := runAll(context.Background(), c, len(ds), 4, true, func(i int) jev.Request {
		return jev.Request{State: spikeState(ds[i], reverse), Questions: spikeQuestions(ds[i]), Model: "clef"}
	})
	if err != nil {
		t.Fatal(err)
	}
	out := spikeRun{}
	for _, r := range res {
		out.answers = append(out.answers, r.Answers)
	}
	return out
}

type spikeSummary struct {
	correct, dominant int
	domConf, conConf  float64
	aucConf, aucEnt   float64
}

func summarizeSpike(ds []spikeDecision, truth map[string]spikeTruth, run spikeRun) spikeSummary {
	var s spikeSummary
	var dom, con, oneMinusConf, ent []float64
	var contested []bool
	for i, d := range ds {
		tr := truth[d.ID]
		pick := run.answers[i]["pick"]
		if tr.Class == "dominant" {
			s.dominant++
			if tr.CorrectOption != nil && pick.Choice == *tr.CorrectOption {
				s.correct++
			}
			dom = append(dom, pick.Confidence)
		} else {
			con = append(con, pick.Confidence)
		}
		contested = append(contested, tr.Class == "contested")
		oneMinusConf = append(oneMinusConf, 1-pick.Confidence)
		ent = append(ent, entropy(pick.Probabilities))
	}
	s.domConf, s.conConf = mean(dom), mean(con)
	s.aucConf, s.aucEnt = auc(oneMinusConf, contested), auc(ent, contested)
	return s
}

func TestClefDecisions(t *testing.T) {
	c := newClefClient(t)
	ds, truth := loadSpike(t)
	fwd := clefSpike(t, c, ds, false)
	rev := clefSpike(t, c, ds, true)

	flips := map[string][2]int{}
	for i, d := range ds {
		cl := truth[d.ID].Class
		f := flips[cl]
		f[1]++
		if fwd.answers[i]["pick"].Choice != rev.answers[i]["pick"].Choice {
			f[0]++
		}
		flips[cl] = f
	}

	row := func(name string, s spikeSummary) []string {
		return []string{name, fmt.Sprintf("%d/%d", s.correct, s.dominant), f2(s.domConf), f2(s.conConf), f2(s.aucConf), f2(s.aucEnt)}
	}
	rows := [][]string{{"model", "dominant correct", "conf dominant", "conf contested", "AUC 1-conf", "AUC entropy"}, row("clef", summarizeSpike(ds, truth, fwd))}
	if jc := jevForClefProbe(t); jc != nil {
		res, err := runAll(context.Background(), jc, len(ds), 4, true, func(i int) jev.Request {
			return jev.Request{State: spikeState(ds[i], false), Questions: spikeQuestions(ds[i])}
		})
		if err != nil {
			t.Fatal(err)
		}
		jr := spikeRun{}
		for _, r := range res {
			jr.answers = append(jr.answers, r.Answers)
		}
		rows = append(rows, row("jev", summarizeSpike(ds, truth, jr)))
	}
	table(t, "clef vs jev on the 40 spike decisions", rows)
	t.Logf("clef pick changes when option order is reversed: dominant %d/%d, contested %d/%d", flips["dominant"][0], flips["dominant"][1], flips["contested"][0], flips["contested"][1])
	if math.IsNaN(summarizeSpike(ds, truth, fwd).aucConf) {
		t.Error("AUC undefined: truth file lacks one class")
	}
}

func jevForClefProbe(t *testing.T) *jev.Client {
	t.Helper()
	key := loadKey(t)
	if key == "" {
		return nil
	}
	c, err := jev.New(jev.WithAPIKey(key), jev.WithTimeout(20*time.Second))
	if err != nil {
		t.Fatalf("new jev client: %v", err)
	}
	t.Cleanup(c.CloseIdleConnections)
	return c
}
