package gojev_test

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"testing"
	"text/tabwriter"
	"time"

	"github.com/kataras/jev"
)

// Live probe: can Jev triage FinFocus-style recommendations?
// Run: go test -run TestJevRecommendations -v -count=1
// Needs TYPESAFE_API_KEY in the environment or in ./.env. Skips otherwise.
// Risk, false-positive labels below were written before any call was made.

type rec struct {
	ID          string            `json:"resourceId"`
	Type        string            `json:"-"`
	Description string            `json:"description"`
	Savings     float64           `json:"estimatedSavings"`
	Currency    string            `json:"currency"`
	Tags        map[string]string `json:"tags,omitempty"`

	risk    int
	falsePo *bool
}

func ptr(b bool) *bool { return &b }

var recs = []rec{
	{ID: "aws:ec2:Instance/i-0abc123def456789", Type: "RIGHTSIZE", Description: "Downsize from m5.xlarge to m5.large - instance is underutilized (avg CPU 15%)", Savings: 87.60, risk: 1},
	{ID: "aws:ec2:Instance/i-0xyz987wvu654321", Type: "TERMINATE", Description: "Terminate idle instance - no traffic for 30+ days", Savings: 175.20, risk: 1},
	{ID: "aws:rds:Instance/db-prod-mysql", Type: "PURCHASE_COMMITMENT", Description: "Switch to Reserved Instance (1-year) for 32% savings", Savings: 156.00, risk: 0},
	{ID: "aws:s3:Bucket/logs-archive", Type: "DELETE_UNUSED", Description: "Delete old log files older than 90 days - 500GB unused storage", Savings: 11.50, risk: 1},
	{ID: "aws:ec2:Volume/vol-0abc123def456789", Type: "MODIFY", Description: "Convert gp2 to gp3 for better price-performance", Savings: 8.00, risk: 0},
	{ID: "kubernetes:deployment/api-server", Type: "ADJUST_REQUESTS", Description: "Reduce CPU requests from 2 to 0.5 cores - actual usage is 0.3 cores", Savings: 45.00, risk: 1},
	{ID: "aws:lambda:Function/data-processor", Type: "RIGHTSIZE", Description: "Reduce memory from 1024MB to 512MB - peak usage 280MB", Savings: 12.30, risk: 0},
	{ID: "aws:ec2:Instance/i-001", Type: "RIGHTSIZE", Description: "Downsize from c5.2xlarge to c5.xlarge", Savings: 156.00, risk: 1},
	{ID: "aws:ec2:Instance/i-002", Type: "TERMINATE", Description: "Terminate idle instance - no traffic 60+ days", Savings: 312.00, risk: 1},
	{ID: "aws:rds:Instance/db-analytics", Type: "PURCHASE_COMMITMENT", Description: "Purchase 1-year reserved instance", Savings: 450.00, risk: 0},
	{ID: "aws:ec2:Instance/i-003", Type: "RIGHTSIZE", Description: "Downsize from m5.4xlarge to m5.2xlarge", Savings: 280.00, risk: 1},
	{ID: "aws:s3:Bucket/old-backups", Type: "DELETE_UNUSED", Description: "Delete 2TB of old backups", Savings: 46.00, risk: 2},
	{ID: "aws:ebs:Volume/vol-001", Type: "MODIFY", Description: "Convert io1 to gp3", Savings: 125.00, risk: 1},
	{ID: "aws:ec2:Instance/i-004", Type: "MIGRATE", Description: "Migrate to Graviton for 40% cost reduction", Savings: 200.00, risk: 2},
	{ID: "kubernetes:deployment/worker-pool", Type: "CONSOLIDATE", Description: "Consolidate 5 small deployments into 2 larger ones", Savings: 180.00, risk: 2},
	{ID: "aws:ec2:Instance/i-005", Type: "SCHEDULE", Description: "Schedule non-prod instances to stop overnight", Savings: 95.00, risk: 0},
	{ID: "kubernetes:deployment/legacy-api", Type: "REFACTOR", Description: "Refactor monolith to serverless for better cost efficiency", Savings: 350.00, risk: 2},
	{ID: "aws:elasticache:Cluster/cache-001", Type: "RIGHTSIZE", Description: "Reduce cache node count from 3 to 2", Savings: 78.00, risk: 1},
	{ID: "aws:nat-gateway/nat-001", Type: "OTHER", Description: "Consider VPC endpoints to reduce NAT gateway costs", Savings: 120.00, risk: 1},
	{ID: "aws:ec2:Instance/i-dr-01", Type: "TERMINATE", Description: "Terminate idle instance - no traffic for 45 days", Savings: 140.00, risk: 2, falsePo: ptr(true),
		Tags: map[string]string{"env": "prod", "role": "dr-standby", "owner": "platform"}},
	{ID: "aws:ebs:Volume/vol-dev-77", Type: "DELETE_UNUSED", Description: "Delete unattached 100GB volume, detached for 120 days", Savings: 10.00, risk: 0, falsePo: ptr(false),
		Tags: map[string]string{"env": "dev", "owner": "sandbox"}},
	{ID: "aws:rds:Instance/db-claims", Type: "TERMINATE", Description: "Terminate unused database - no connections for 60 days", Savings: 220.00, risk: 2, falsePo: ptr(true),
		Tags: map[string]string{"env": "prod", "compliance": "hipaa", "retention": "7y"}},
	{ID: "aws:ec2:Instance/i-dev-12", Type: "SCHEDULE", Description: "Stop instance nightly, idle 8pm-7am", Savings: 40.00, risk: 0, falsePo: ptr(false),
		Tags: map[string]string{"env": "dev", "owner": "sandbox"}},
}

var actionTypes = map[string]string{
	"RIGHTSIZE":           "Change the size or capacity of an existing resource to match its actual usage",
	"TERMINATE":           "Shut down and remove an idle resource that is not doing useful work",
	"PURCHASE_COMMITMENT": "Buy a reserved instance, savings plan or committed-use discount",
	"ADJUST_REQUESTS":     "Change Kubernetes resource requests or limits",
	"MODIFY":              "Change a setting or type of a resource in place, for example a storage class",
	"DELETE_UNUSED":       "Delete stored data or detached resources that nothing uses",
	"MIGRATE":             "Move a workload to a different architecture, platform or region",
	"CONSOLIDATE":         "Merge several resources into fewer ones",
	"SCHEDULE":            "Run a resource only during the hours it is needed",
	"REFACTOR":            "Redesign the application or architecture",
	"OTHER":               "Anything that fits none of the other actions",
	"INVESTIGATE":         "Needs a human to look into it before any action can be chosen",
}

var urgencyLevels = []string{
	"Ignore: negligible benefit, or the risk outweighs the saving",
	"Low: small saving, do it when convenient",
	"Medium: worthwhile saving, schedule it this sprint",
	"High: large saving with manageable risk, act this week",
	"Critical: large recurring waste with low risk, act today",
}

type result struct {
	rec      rec
	safe     float64
	urgency  float64
	urgConf  float64
	action   string
	actConf  float64
	falsePos float64
	latency  time.Duration
	tokens   int
}

func loadKey(t *testing.T) string {
	t.Helper()
	if k := strings.TrimSpace(os.Getenv("TYPESAFE_API_KEY")); k != "" {
		return k
	}
	f, err := os.Open(".env")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if k, v, ok := strings.Cut(strings.TrimSpace(sc.Text()), "="); ok && k == "TYPESAFE_API_KEY" {
			return strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return ""
}

func newClient(t *testing.T) *jev.Client {
	t.Helper()
	key := loadKey(t)
	if key == "" {
		t.Skip("TYPESAFE_API_KEY not set")
	}
	c, err := jev.New(jev.WithAPIKey(key), jev.WithTimeout(20*time.Second))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Cleanup(c.CloseIdleConnections)
	return c
}

func questions() jev.Questions {
	return jev.Questions{
		"safe": jev.Noul{
			Instructions: "Can this recommendation be applied to a production workload without risking downtime or data loss?",
			Criteria: &jev.NoulCriteria{
				True:  "Reversible or no impact on running workloads; safe to apply",
				False: "Could cause downtime, data loss, or an irreversible change",
			},
		},
		"urgency": jev.Score{
			Instructions: "How urgently should an engineering team act on this cost recommendation?",
			Criteria:     urgencyLevels,
		},
		"action": jev.Choice{
			Instructions: "Which kind of action does this recommendation describe?",
			Criteria:     actionTypes,
		},
		"false_positive": jev.Noul{
			Instructions: "Is the resource probably provisioned on purpose, for example as a standby, for compliance or for peak load, so that this recommendation is a false positive?",
			Criteria: &jev.NoulCriteria{
				True:  "The resource has a legitimate reason to look idle or oversized",
				False: "The resource really is wasteful",
			},
		},
	}
}

func ask(ctx context.Context, c *jev.Client, r rec) (result, error) {
	start := time.Now()
	resp, err := c.SystemOne(ctx, jev.Request{State: r, Questions: questions()})
	if err != nil {
		return result{}, err
	}
	out := result{rec: r, latency: time.Since(start), tokens: resp.Usage.InputTokens}
	safe, ok1 := resp.Noul("safe")
	urg, ok2 := resp.Score("urgency")
	act, ok3 := resp.Choice("action")
	fp, ok4 := resp.Noul("false_positive")
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return result{}, fmt.Errorf("missing answers in response: %s", resp.Raw())
	}
	out.safe, out.urgency, out.urgConf = safe.Noul, urg.Score, urg.Confidence
	out.action, out.actConf, out.falsePos = act.Choice, act.Confidence, fp.Noul
	return out, nil
}

func TestJevRecommendationsModels(t *testing.T) {
	c := newClient(t)
	models, err := c.ListModels(context.Background())
	if err != nil {
		t.Fatalf("list models: %v", err)
	}
	for _, m := range models {
		t.Logf("model %s (%s)", m.Name, m.Description)
	}
	if len(models) == 0 {
		t.Fatal("no models returned")
	}
}

func TestJevRecommendationsBadKey(t *testing.T) {
	c, err := jev.New(jev.WithAPIKey("apikey_invalid_probe"), jev.WithTimeout(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.SystemOne(context.Background(), jev.Request{State: "x", Questions: jev.Questions{"q": jev.Noul{Instructions: "Is this text empty?"}}})
	var apiErr *jev.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *jev.APIError, got %T %v", err, err)
	}
	t.Logf("bad key -> status %d, unauthorized=%v forbidden=%v, requestID=%q, msg=%q",
		apiErr.Status, errors.Is(err, jev.ErrUnauthorized), errors.Is(err, jev.ErrForbidden), apiErr.RequestID, apiErr.Message)
}

func TestJevRecommendations(t *testing.T) {
	c := newClient(t)
	ctx := context.Background()

	results := make([]result, 0, len(recs))
	for _, r := range recs {
		res, err := ask(ctx, c, r)
		if err != nil {
			t.Fatalf("%s: %v", r.ID, err)
		}
		results = append(results, res)
	}

	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "resource\tdeclared\tchosen\tconf\tsafe\turgency\tfalsepos\texpRisk\tms")
	for _, r := range results {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%.2f\t%.2f\t%.2f\t%.2f\t%d\t%d\n",
			r.rec.ID[strings.LastIndex(r.rec.ID, ":")+1:], r.rec.Type, r.action, r.actConf, r.safe, r.urgency, r.falsePos, r.rec.risk, r.latency.Milliseconds())
	}
	tw.Flush()
	t.Logf("\n%s", buf.String())

	t.Run("risk ordering", func(t *testing.T) {
		var lo, hi []float64
		var x, y []float64
		for _, r := range results {
			unsafe := 1 - r.safe
			x = append(x, float64(r.rec.risk))
			y = append(y, unsafe)
			switch r.rec.risk {
			case 0:
				lo = append(lo, unsafe)
			case 2:
				hi = append(hi, unsafe)
			}
		}
		rho := spearman(x, y)
		t.Logf("mean P(unsafe): expected-low=%.2f (n=%d) expected-high=%.2f (n=%d), spearman=%.2f", mean(lo), len(lo), mean(hi), len(hi), rho)
		if mean(hi)-mean(lo) < 0.15 {
			t.Errorf("high-risk group not separated from low-risk group: gap %.2f", mean(hi)-mean(lo))
		}
	})

	t.Run("false positive from tags", func(t *testing.T) {
		var yes, no []float64
		var untagged []float64
		for _, r := range results {
			switch {
			case r.rec.falsePo == nil:
				untagged = append(untagged, r.falsePos)
			case *r.rec.falsePo:
				yes = append(yes, r.falsePos)
			default:
				no = append(no, r.falsePos)
			}
		}
		t.Logf("mean P(false positive): tagged-legit=%.2f tagged-wasteful=%.2f untagged=%.2f", mean(yes), mean(no), mean(untagged))
		if mean(yes)-mean(no) < 0.2 {
			t.Errorf("tags did not shift false-positive odds enough: gap %.2f", mean(yes)-mean(no))
		}
	})

	t.Run("action classification", func(t *testing.T) {
		agree, total := 0, 0
		var misses []string
		for _, r := range results {
			if r.rec.Type == "OTHER" {
				t.Logf("declared OTHER %s -> chosen %s (conf %.2f)", r.rec.ID, r.action, r.actConf)
				continue
			}
			total++
			if r.action == r.rec.Type {
				agree++
			} else {
				misses = append(misses, fmt.Sprintf("%s declared=%s chosen=%s", r.rec.ID, r.rec.Type, r.action))
			}
		}
		t.Logf("agreement with plugin-declared type: %d/%d", agree, total)
		for _, m := range misses {
			t.Log("  miss:", m)
		}
		if float64(agree)/float64(total) < 0.5 {
			t.Errorf("agreement below 50%%: %d/%d", agree, total)
		}
	})

	t.Run("urgency vs savings", func(t *testing.T) {
		var s, u []float64
		for _, r := range results {
			s = append(s, r.rec.Savings)
			u = append(u, r.urgency)
		}
		t.Logf("spearman(savings, urgency)=%.2f", spearman(s, u))
	})

	t.Run("latency and cost", func(t *testing.T) {
		var ms []float64
		tokens := 0
		for _, r := range results {
			ms = append(ms, float64(r.latency.Milliseconds()))
			tokens += r.tokens
		}
		sort.Float64s(ms)
		p95 := ms[int(math.Ceil(0.95*float64(len(ms))))-1]
		t.Logf("latency ms: p50=%.0f p95=%.0f max=%.0f; input tokens total=%d avg=%d (~$%.5f)",
			ms[len(ms)/2], p95, ms[len(ms)-1], tokens, tokens/len(results), float64(tokens)/1e6*0.042)
		if p95 > 3000 {
			t.Errorf("p95 latency %.0fms exceeds 3s", p95)
		}
	})
}

func TestJevRecommendationsStability(t *testing.T) {
	c := newClient(t)
	ctx := context.Background()
	const repeats = 10
	for _, idx := range []int{1, 11, 19} {
		r := recs[idx]
		var safe, urg, fp []float64
		actions := map[string]int{}
		for range repeats {
			res, err := ask(ctx, c, r)
			if err != nil {
				t.Fatalf("%s: %v", r.ID, err)
			}
			safe = append(safe, res.safe)
			urg = append(urg, res.urgency)
			fp = append(fp, res.falsePos)
			actions[res.action]++
		}
		t.Logf("%s x%d: safe spread=%.4f urgency spread=%.4f falsepos spread=%.4f actions=%v",
			r.ID, repeats, spread(safe), spread(urg), spread(fp), actions)
		if s := spread(safe); s > 0.05 {
			t.Errorf("%s: safe probability unstable, spread %.3f", r.ID, s)
		}
		if len(actions) > 1 {
			t.Errorf("%s: action choice flipped across identical calls: %v", r.ID, actions)
		}
	}
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return math.NaN()
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func spread(v []float64) float64 {
	lo, hi := v[0], v[0]
	for _, x := range v {
		lo, hi = math.Min(lo, x), math.Max(hi, x)
	}
	return hi - lo
}

func ranks(v []float64) []float64 {
	idx := make([]int, len(v))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool { return v[idx[a]] < v[idx[b]] })
	r := make([]float64, len(v))
	for i := 0; i < len(idx); {
		j := i
		for j+1 < len(idx) && v[idx[j+1]] == v[idx[i]] {
			j++
		}
		avg := float64(i+j)/2 + 1
		for k := i; k <= j; k++ {
			r[idx[k]] = avg
		}
		i = j + 1
	}
	return r
}

func spearman(x, y []float64) float64 {
	rx, ry := ranks(x), ranks(y)
	mx, my := mean(rx), mean(ry)
	var num, dx, dy float64
	for i := range rx {
		num += (rx[i] - mx) * (ry[i] - my)
		dx += (rx[i] - mx) * (rx[i] - mx)
		dy += (ry[i] - my) * (ry[i] - my)
	}
	if dx == 0 || dy == 0 {
		return math.NaN()
	}
	return num / math.Sqrt(dx*dy)
}
