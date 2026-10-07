//go:build probe

package gojev_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kataras/jev"
)

const (
	pricePerMTok = 0.042
	spendCapUSD  = 15.0
	cacheDir     = ".probe-cache"
)

var spentMicroUSD atomic.Int64

type rawAnswer struct {
	Type          string             `json:"type"`
	Noul          float64            `json:"noul"`
	Score         float64            `json:"score"`
	Confidence    float64            `json:"confidence"`
	Choice        string             `json:"choice"`
	Probabilities map[string]float64 `json:"probabilities"`
}

type callResult struct {
	Answers map[string]rawAnswer
	Tokens  int
	Latency time.Duration
	Cached  bool
}

type cacheEntry struct {
	Raw       json.RawMessage `json:"raw"`
	LatencyMS int64           `json:"latency_ms"`
}

type wireResponse struct {
	Answers map[string]rawAnswer `json:"answers"`
	Usage   struct {
		InputTokens int `json:"input_tokens"`
	} `json:"usage"`
}

func spentUSD() float64 { return float64(spentMicroUSD.Load()) / 1e6 }

func call(ctx context.Context, c *jev.Client, req jev.Request, useCache bool) (callResult, error) {
	body, err := json.Marshal(struct {
		S any           `json:"s"`
		Q jev.Questions `json:"q"`
		M string        `json:"m"`
	}{req.State, req.Questions, req.Model})
	if err != nil {
		return callResult{}, err
	}
	sum := sha256.Sum256(body)
	path := filepath.Join(cacheDir, hex.EncodeToString(sum[:8])+".json")
	if useCache {
		if b, err := os.ReadFile(path); err == nil {
			var e cacheEntry
			var w wireResponse
			if json.Unmarshal(b, &e) == nil && json.Unmarshal(e.Raw, &w) == nil {
				return callResult{Answers: w.Answers, Tokens: w.Usage.InputTokens, Latency: time.Duration(e.LatencyMS) * time.Millisecond, Cached: true}, nil
			}
		}
	}
	if spentUSD() > spendCapUSD {
		return callResult{}, fmt.Errorf("spend cap $%.2f reached", spendCapUSD)
	}
	start := time.Now()
	resp, err := c.SystemOne(ctx, req)
	lat := time.Since(start)
	if err != nil {
		return callResult{}, err
	}
	spentMicroUSD.Add(int64(float64(resp.Usage.InputTokens) * pricePerMTok))
	var w wireResponse
	if err := json.Unmarshal(resp.Raw(), &w); err != nil {
		return callResult{}, err
	}
	if useCache {
		_ = os.MkdirAll(cacheDir, 0o700)
		b, _ := json.Marshal(cacheEntry{Raw: resp.Raw(), LatencyMS: lat.Milliseconds()})
		_ = os.WriteFile(path, b, 0o600)
	}
	return callResult{Answers: w.Answers, Tokens: w.Usage.InputTokens, Latency: lat}, nil
}

type item = map[string]any

func loadJSON[T any](t *testing.T, path string) T {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("missing %s: %v", path, err)
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return v
}

func idOf(r item) string { s, _ := r["id"].(string); return s }

func sub(r item, key string) item {
	m, _ := r[key].(map[string]any)
	return m
}

func clone(r item) item {
	b, _ := json.Marshal(r)
	var out item
	_ = json.Unmarshal(b, &out)
	return out
}

func without(r item, paths ...[]string) item {
	out := clone(r)
	for _, p := range paths {
		cur := out
		for i, k := range p {
			if i == len(p)-1 {
				delete(cur, k)
				break
			}
			next, ok := cur[k].(map[string]any)
			if !ok {
				break
			}
			cur = next
		}
	}
	return out
}

type label struct {
	ID       string `json:"id"`
	Safe     bool   `json:"safe_unattended"`
	FalsePos bool   `json:"false_positive"`
	Priority int    `json:"priority"`
	ActFit   bool   `json:"action_fit"`
}

func labelMap(ls []label) map[string]label {
	m := map[string]label{}
	for _, l := range ls {
		m[l.ID] = l
	}
	return m
}

func runAll(ctx context.Context, c *jev.Client, n, workers int, useCache bool, build func(i int) jev.Request) ([]callResult, error) {
	out := make([]callResult, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	for i := range n {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			out[i], errs[i] = call(ctx, c, build(i), useCache)
		}()
	}
	wg.Wait()
	for _, e := range errs {
		if e != nil {
			return out, e
		}
	}
	return out, nil
}

func auc(scores []float64, positive []bool) float64 {
	type sp struct {
		s float64
		p bool
	}
	xs := make([]sp, len(scores))
	for i := range scores {
		xs[i] = sp{scores[i], positive[i]}
	}
	sort.Slice(xs, func(a, b int) bool { return xs[a].s < xs[b].s })
	var rankSum float64
	var np, nn float64
	for i := 0; i < len(xs); {
		j := i
		for j+1 < len(xs) && xs[j+1].s == xs[i].s {
			j++
		}
		avg := float64(i+j)/2 + 1
		for k := i; k <= j; k++ {
			if xs[k].p {
				rankSum += avg
				np++
			} else {
				nn++
			}
		}
		i = j + 1
	}
	if np == 0 || nn == 0 {
		return math.NaN()
	}
	return (rankSum - np*(np+1)/2) / (np * nn)
}

func kappa(a, b []bool) float64 {
	var n, agree, ay, by float64
	for i := range a {
		n++
		if a[i] == b[i] {
			agree++
		}
		if a[i] {
			ay++
		}
		if b[i] {
			by++
		}
	}
	po := agree / n
	pe := (ay/n)*(by/n) + (1-ay/n)*(1-by/n)
	if pe == 1 {
		return math.NaN()
	}
	return (po - pe) / (1 - pe)
}
