package decision

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"net/http"
	"strings"
	"testing"

	"github.com/kataras/jev"
	"github.com/rshade/ax-go/contract"

	"github.com/rshade/jev-decide/jevclient"
)

func scoreBody(score, confidence float64) string {
	return fmt.Sprintf(`{"model":"jev-latest","answers":{"score":{"type":"score","score":%v,"confidence":%v,`+
		`"legend":{"0":"minor","1":"noticeable","2":"high"},`+
		`"probabilities":{"0":0.05,"1":0.15,"2":0.8}}},`+
		`"usage":{"input_tokens":5,"output_tokens":1}}`, score, confidence)
}

func sampleRateQuestion(t *testing.T) RateQuestion[grade] {
	t.Helper()
	return RateQuestion[grade]{
		State:        "the page is down for some users",
		Instructions: "How severe is this incident?",
		Levels:       sampleLevels(t),
	}
}

func scoreClassOf(r ScoreResult[grade]) string {
	switch r.(type) {
	case ScoreDecided[grade]:
		return "decided"
	case ScoreUncertain[grade]:
		return "uncertain"
	case ScoreEscalate[grade]:
		return "escalate"
	default:
		return "unknown"
	}
}

func TestRateTurnsConfidenceIntoTheRightCase(t *testing.T) {
	wide := mustThresholds(t, 0.4, 0.8)
	tests := []struct {
		name       string
		confidence float64
		opts       []ChooseOption
		want       string
	}{
		{"confident", 0.95, nil, "decided"},
		{"exactly the confident level", 0.9, nil, "decided"},
		{"between the levels", 0.7, nil, "uncertain"},
		{"exactly the floor", 0.5, nil, "uncertain"},
		{"below the floor", 0.3, nil, "escalate"},
		{"custom thresholds decide", 0.85, []ChooseOption{WithThresholds(wide)}, "decided"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _, _ := answerWith(t, scoreBody(1.9, tt.confidence))

			res, err := Rate(context.Background(), client, sampleRateQuestion(t), tt.opts...)

			if err != nil {
				t.Fatalf("Rate error: %v", err)
			}
			if got := scoreClassOf(res); got != tt.want {
				t.Fatalf("confidence %v gave %q, want %q", tt.confidence, got, tt.want)
			}
			if res.Score() != 1.9 || res.Nearest() != high || res.Confidence().Float64() != tt.confidence {
				t.Errorf("result = score %v nearest %q confidence %v, want 1.9 high %v",
					res.Score(), res.Nearest(), res.Confidence().Float64(), tt.confidence)
			}
			probs := res.Probabilities()
			if len(probs) != 3 || probs[low].Float64() != 0.05 || probs[high].Float64() != 0.8 {
				t.Errorf("Probabilities() = %v, want a probability for each of the three levels", probs)
			}
			if d, ok := res.(ScoreDecided[grade]); ok && d.Level() != high {
				t.Errorf("Level() = %q, want high", d.Level())
			}
		})
	}
}

func TestRateNearestLevelRoundsHalvesDown(t *testing.T) {
	tests := []struct {
		score float64
		want  grade
	}{
		{0, low}, {0.49, low}, {0.5, low}, {0.51, mid}, {1, mid}, {1.5, mid}, {1.51, high}, {2, high},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.score), func(t *testing.T) {
			client, _, _ := answerWith(t, scoreBody(tt.score, 0.95))

			res, err := Rate(context.Background(), client, sampleRateQuestion(t))

			if err != nil || res.Nearest() != tt.want || res.Score() != tt.score {
				t.Fatalf("Rate = nearest %v score %v, %v; want %s and the unchanged score", res.Nearest(), res.Score(), err, tt.want)
			}
		})
	}
}

func TestRateSendsTheRubricAndInstructions(t *testing.T) {
	client, _, sent := answerWith(t, scoreBody(1, 0.95))

	if _, err := Rate(context.Background(), client, sampleRateQuestion(t)); err != nil {
		t.Fatalf("Rate error: %v", err)
	}

	var req struct {
		State     string `json:"state"`
		Questions map[string]struct {
			Type         string   `json:"type"`
			Instructions string   `json:"instructions"`
			Criteria     []string `json:"criteria"`
		} `json:"questions"`
	}
	if err := json.Unmarshal(*sent, &req); err != nil {
		t.Fatalf("decoding request %s: %v", *sent, err)
	}
	q := req.Questions["score"]
	if q.Type != "score" || q.Instructions != "How severe is this incident?" ||
		strings.Join(q.Criteria, "|") != "minor|noticeable|high" {
		t.Fatalf("request question = %+v, want the score type, instructions and ordered criteria", q)
	}
}

func TestRateRejectsInvalidInputBeforeAnyRequest(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(q *RateQuestion[grade]) []ChooseOption
		wantErr error
	}{
		{"zero levels", func(q *RateQuestion[grade]) []ChooseOption { q.Levels = Levels[grade]{}; return nil }, ErrInvalidLevels},
		{"nil state", func(q *RateQuestion[grade]) []ChooseOption { q.State = nil; return nil }, ErrInvalidQuestion},
		{"oversized state", func(q *RateQuestion[grade]) []ChooseOption {
			q.State = strings.Repeat("a", 4*maxStateTokens+8)
			return nil
		}, ErrInvalidQuestion},
		{"zero thresholds", func(*RateQuestion[grade]) []ChooseOption {
			return []ChooseOption{WithThresholds(Thresholds{})}
		}, ErrInvalidThresholds},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, hits := newServer(t, func(http.ResponseWriter, *http.Request) {})
			client := newClient(t, srv)
			q := sampleRateQuestion(t)
			opts := tt.mutate(&q)

			res, err := Rate(context.Background(), client, q, opts...)

			if !errors.Is(err, tt.wantErr) || res != nil {
				t.Fatalf("Rate = %v, %v; want %v and no result", res, err, tt.wantErr)
			}
			if got := hits.Load(); got != 0 {
				t.Fatalf("server received %d requests, want 0", got)
			}
		})
	}
}

func TestRateRejectsANilClient(t *testing.T) {
	res, err := Rate(context.Background(), nil, sampleRateQuestion(t))

	if !errors.Is(err, ErrNilClient) || res != nil {
		t.Fatalf("Rate(nil client) = %v, %v; want ErrNilClient and no result", res, err)
	}
}

func TestRateReturnsUnusableAnswersAsErrorsNeverResults(t *testing.T) {
	missing := `{"model":"m","answers":{"score":{"type":"score","score":1,"confidence":0.9,` +
		`"legend":{"0":"a","1":"b","2":"c"},"probabilities":{"0":0.1,"1":0.2,"2":0.7}}},"usage":{"input_tokens":1,"output_tokens":1}}`
	tests := []struct {
		name string
		body string
	}{
		{"score above the rubric", scoreBody(2.5, 0.95)},
		{"score below the rubric", scoreBody(-0.5, 0.95)},
		{"confidence above one", scoreBody(1, 1.4)},
		{"no answer to the question", `{"model":"m","answers":{"other":{"type":"noul","noul":0.5}},"usage":{"input_tokens":1,"output_tokens":1}}`},
		{"no score probability for a level", strings.Replace(missing, `"2":0.7`, `"9":0.7`, 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _, _ := answerWith(t, tt.body)

			res, err := Rate(context.Background(), client, sampleRateQuestion(t))

			var ce *contract.Error
			if !errors.As(err, &ce) || ce.ErrorCode != jevclient.CodeInvalidResponse {
				t.Fatalf("Rate error = %v, want a %s error", err, jevclient.CodeInvalidResponse)
			}
			if res != nil {
				t.Fatalf("Rate returned %v alongside an error, want no result", res)
			}
		})
	}
}

func TestScoreResultsDoNotPresentANonDecisionAsALevel(t *testing.T) {
	p, _ := NewProbability(0.6)
	for _, k := range []kind{kindUncertain, kindEscalate} {
		r := newScoreResult(k, 1.2, mid, p, map[grade]Probability{mid: p})
		if _, ok := r.(ScoreDecided[grade]); ok {
			t.Errorf("a %v score result is a ScoreDecided", k)
		}
		if s := fmt.Sprint(r); strings.Contains(s, "level=") {
			t.Errorf("%v prints %q, which presents the nearest level as a level", k, s)
		}
	}
	if s := fmt.Sprint(newScoreResult(kindDecided, 1.9, high, p, nil)); !strings.Contains(s, "level=high") {
		t.Errorf("a decided score prints %q, want level=high", s)
	}
}

func TestMatchScoreRunsTheHandlerForTheCase(t *testing.T) {
	p, _ := NewProbability(0.9)
	handlers := func(r ScoreResult[grade]) string {
		return MatchScore(r,
			func(d ScoreDecided[grade]) string { return "decided " + string(d.Level()) },
			func(u ScoreUncertain[grade]) string { return "uncertain " + string(u.Nearest()) },
			func(e ScoreEscalate[grade]) string { return "escalate " + string(e.Nearest()) },
		)
	}
	tests := []struct {
		r    ScoreResult[grade]
		want string
	}{
		{newScoreResult(kindDecided, 2, high, p, nil), "decided high"},
		{newScoreResult(kindUncertain, 1, mid, p, nil), "uncertain medium"},
		{newScoreResult(kindEscalate, 0, low, p, nil), "escalate low"},
		{nil, "escalate "},
	}
	for _, tt := range tests {
		if got := handlers(tt.r); got != tt.want {
			t.Errorf("MatchScore(%v) = %q, want %q", tt.r, got, tt.want)
		}
	}
}

func TestScoreFromAnswerRejectsWhatTheClientShouldHaveCaught(t *testing.T) {
	good := jev.ScoreAnswer{Score: 1, Confidence: 0.9, Probabilities: map[string]float64{"0": 0.1, "1": 0.2, "2": 0.7}}
	with := func(edit func(*jev.ScoreAnswer)) jev.ScoreAnswer {
		a := good
		a.Probabilities = maps.Clone(good.Probabilities)
		edit(&a)
		return a
	}
	tests := []struct {
		name   string
		answer jev.ScoreAnswer
	}{
		{"NaN score", with(func(a *jev.ScoreAnswer) { a.Score = math.NaN() })},
		{"score above the rubric", with(func(a *jev.ScoreAnswer) { a.Score = 2.1 })},
		{"confidence above one", with(func(a *jev.ScoreAnswer) { a.Confidence = 1.1 })},
		{"missing probability", with(func(a *jev.ScoreAnswer) { delete(a.Probabilities, "2") })},
		{"probability above one", with(func(a *jev.ScoreAnswer) { a.Probabilities["1"] = 1.5 })},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := scoreFromAnswer(context.Background(), tt.answer, sampleLevels(t), DefaultThresholds())

			var ce *contract.Error
			if !errors.As(err, &ce) || ce.ErrorCode != jevclient.CodeInvalidResponse || res != nil {
				t.Fatalf("scoreFromAnswer = %v, %v; want a %s error and no result", res, err, jevclient.CodeInvalidResponse)
			}
		})
	}

	res, err := scoreFromAnswer(context.Background(), good, sampleLevels(t), DefaultThresholds())
	if err != nil || res.Nearest() != mid {
		t.Fatalf("scoreFromAnswer(good) = %v, %v; want the medium level", res, err)
	}
}
