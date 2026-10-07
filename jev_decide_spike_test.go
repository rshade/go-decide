//go:build probe

package gojev_test

import (
	"context"
	"fmt"
	"math"
	"sort"
	"testing"

	"github.com/kataras/jev"
)

// Spike: does Jev behave usefully on decision-shaped input, and does its
// confidence separate clear-cut decisions from contested ones?
// Run: go test -run TestSpike -v -count=1

const (
	decisionsPath      = "testdata/decisions.json"
	decisionsTruthPath = "testdata/decisions_truth.json"
)

type spikeOption struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Pros            []string `json:"pros"`
	Cons            []string `json:"cons"`
	CostUSDPerMonth *float64 `json:"cost_usd_per_month"`
	TimeToShipWeeks *float64 `json:"time_to_ship_weeks"`
}

type spikeDecision struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Context     string        `json:"context"`
	Constraints []string      `json:"constraints"`
	Options     []spikeOption `json:"options"`
}

type spikeTruth struct {
	ID            string  `json:"id"`
	Class         string  `json:"class"`
	CorrectOption *string `json:"correct_option"`
}

func spikeQuestions(d spikeDecision) jev.Questions {
	choices := map[string]string{}
	for _, o := range d.Options {
		choices[o.Name] = o.Description
	}
	return jev.Questions{
		"pick": jev.Choice{
			Instructions: "Given the stated constraints, which option should the team choose?",
			Criteria:     choices,
		},
		"clear_winner": jev.Noul{
			Instructions: "Is one option clearly the best choice given the constraints, so that reasonable senior engineers would agree on it?",
			Criteria: &jev.NoulCriteria{
				True:  "One option is clearly best and experts would agree",
				False: "Several options are defensible and experts could reasonably disagree",
			},
		},
		"disagreement": jev.Noul{
			Instructions: "Would reasonable senior engineers disagree about which option is best here?",
		},
	}
}

func spikeState(d spikeDecision, reverse bool) any {
	opts := make([]any, 0, len(d.Options))
	for i := range d.Options {
		j := i
		if reverse {
			j = len(d.Options) - 1 - i
		}
		opts = append(opts, d.Options[j])
	}
	return item{"title": d.Title, "context": d.Context, "constraints": d.Constraints, "options": opts}
}

func entropy(p map[string]float64) float64 {
	var h float64
	for _, v := range p {
		if v > 0 {
			h -= v * math.Log2(v)
		}
	}
	return h
}

type spikeRun struct {
	answers []map[string]rawAnswer
}

func runSpike(t *testing.T, c *jev.Client, ds []spikeDecision, reverse, useCache bool) spikeRun {
	t.Helper()
	res, err := runAll(context.Background(), c, len(ds), 8, useCache, func(i int) jev.Request {
		return jev.Request{State: spikeState(ds[i], reverse), Questions: spikeQuestions(ds[i])}
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

func loadSpike(t *testing.T) ([]spikeDecision, map[string]spikeTruth) {
	ds := loadJSON[[]spikeDecision](t, decisionsPath)
	tr := map[string]spikeTruth{}
	for _, x := range loadJSON[[]spikeTruth](t, decisionsTruthPath) {
		tr[x.ID] = x
	}
	return ds, tr
}

func TestSpikeDecisions(t *testing.T) {
	c := newClient(t)
	ds, truth := loadSpike(t)
	run := runSpike(t, c, ds, false, true)

	var correct, dominant int
	var misses []string
	byClass := map[string][][3]float64{}
	var contested []bool
	sig := map[string][]float64{"1-confidence": nil, "entropy": nil, "1-maxprob": nil, "disagreement noul": nil, "1-clear_winner noul": nil}
	for i, d := range ds {
		tr := truth[d.ID]
		a := run.answers[i]
		pick := a["pick"]
		maxp := 0.0
		for _, p := range pick.Probabilities {
			maxp = math.Max(maxp, p)
		}
		if tr.Class == "dominant" {
			dominant++
			if tr.CorrectOption != nil && pick.Choice == *tr.CorrectOption {
				correct++
			} else {
				misses = append(misses, fmt.Sprintf("%s picked %q want %v (conf %.2f)", d.ID, pick.Choice, deref(tr.CorrectOption), pick.Confidence))
			}
		}
		contested = append(contested, tr.Class == "contested")
		sig["1-confidence"] = append(sig["1-confidence"], 1-pick.Confidence)
		sig["entropy"] = append(sig["entropy"], entropy(pick.Probabilities))
		sig["1-maxprob"] = append(sig["1-maxprob"], 1-maxp)
		sig["disagreement noul"] = append(sig["disagreement noul"], a["disagreement"].Noul)
		sig["1-clear_winner noul"] = append(sig["1-clear_winner noul"], 1-a["clear_winner"].Noul)
		byClass[tr.Class] = append(byClass[tr.Class], [3]float64{pick.Confidence, a["clear_winner"].Noul, a["disagreement"].Noul})
	}
	t.Logf("accuracy on dominant decisions: %d/%d", correct, dominant)
	for _, m := range misses {
		t.Log("  miss:", m)
	}
	rows := [][]string{{"class", "n", "mean pick confidence", "mean clear_winner", "mean disagreement"}}
	for _, k := range []string{"dominant", "contested"} {
		var c0, c1, c2 []float64
		for _, v := range byClass[k] {
			c0, c1, c2 = append(c0, v[0]), append(c1, v[1]), append(c2, v[2])
		}
		rows = append(rows, []string{k, fmt.Sprint(len(byClass[k])), f2(mean(c0)), f2(mean(c1)), f2(mean(c2))})
	}
	table(t, "signals by designed class", rows)
	names := make([]string, 0, len(sig))
	for k := range sig {
		names = append(names, k)
	}
	sort.Strings(names)
	rows = [][]string{{"signal", "AUC for detecting contested"}}
	for _, k := range names {
		rows = append(rows, []string{k, f2(auc(sig[k], contested))})
	}
	table(t, "contested detection", rows)
}

func TestSpikeOrderAndRepeats(t *testing.T) {
	c := newClient(t)
	ds, truth := loadSpike(t)
	fwd := runSpike(t, c, ds, false, true)
	rev := runSpike(t, c, ds, true, true)
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
	t.Logf("pick changes when option order is reversed: dominant %d/%d, contested %d/%d", flips["dominant"][0], flips["dominant"][1], flips["contested"][0], flips["contested"][1])

	const reps = 3
	base := runSpike(t, c, ds, false, false)
	unstable := map[string][2]int{}
	for range reps - 1 {
		again := runSpike(t, c, ds, false, false)
		for i, d := range ds {
			cl := truth[d.ID].Class
			f := unstable[cl]
			f[1]++
			if base.answers[i]["pick"].Choice != again.answers[i]["pick"].Choice {
				f[0]++
			}
			unstable[cl] = f
		}
	}
	t.Logf("pick changes across identical repeats: dominant %d/%d, contested %d/%d (spend $%.4f)", unstable["dominant"][0], unstable["dominant"][1], unstable["contested"][0], unstable["contested"][1], spentUSD())
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}
