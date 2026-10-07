package decision

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kataras/jev"
	"github.com/rshade/ax-go/contract"

	"github.com/rshade/go-decide/jevclient"
)

func setEnv(t *testing.T, baseURL string) {
	t.Helper()
	t.Setenv("TYPESAFE_API_KEY", "test-key")
	t.Setenv("TYPESAFE_BASE_URL", baseURL)
	t.Setenv("TYPESAFE_DEFAULT_MODEL", "")
	t.Setenv("TYPESAFE_LOG_LEVEL", "")
}

func newServer(t *testing.T, h http.HandlerFunc) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func newClient(t *testing.T, srv *httptest.Server) *jev.Client {
	t.Helper()
	setEnv(t, srv.URL)
	c, err := jevclient.NewClient()
	if err != nil {
		t.Fatalf("jevclient.NewClient() error: %v", err)
	}
	return c
}

func sampleOptions(t *testing.T) Options[action] {
	t.Helper()
	o, err := NewOptions(map[action]string{ship: "release it", hold: "wait", kill: "drop it"})
	if err != nil {
		t.Fatalf("NewOptions error: %v", err)
	}
	return o
}

func sampleQuestion(t *testing.T) Question[action] {
	t.Helper()
	return Question[action]{
		State:        "the release candidate passed every check",
		Instructions: "What should we do with this release?",
		Options:      sampleOptions(t),
	}
}

func TestChooseRejectsInvalidInputBeforeAnyRequest(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(q *Question[action]) []ChooseOption
		wantErr error
		wantMsg string
	}{
		{
			name: "zero options",
			mutate: func(q *Question[action]) []ChooseOption {
				q.Options = Options[action]{}
				return nil
			},
			wantErr: ErrInvalidOptions,
			wantMsg: "NewOptions",
		},
		{
			name: "nil state",
			mutate: func(q *Question[action]) []ChooseOption {
				q.State = nil
				return nil
			},
			wantErr: ErrInvalidQuestion,
			wantMsg: "State",
		},
		{
			name: "empty state",
			mutate: func(q *Question[action]) []ChooseOption {
				q.State = map[string]any{}
				return nil
			},
			wantErr: ErrInvalidQuestion,
			wantMsg: "State",
		},
		{
			name: "unencodable state",
			mutate: func(q *Question[action]) []ChooseOption {
				q.State = make(chan int)
				return nil
			},
			wantErr: ErrInvalidQuestion,
			wantMsg: "encoded",
		},
		{
			name: "oversized state",
			mutate: func(q *Question[action]) []ChooseOption {
				q.State = strings.Repeat("a", 4*maxStateTokens+8)
				return nil
			},
			wantErr: ErrInvalidQuestion,
			wantMsg: "too large",
		},
		{
			name: "zero thresholds",
			mutate: func(*Question[action]) []ChooseOption {
				return []ChooseOption{WithThresholds(Thresholds{})}
			},
			wantErr: ErrInvalidThresholds,
			wantMsg: "NewThresholds",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, hits := newServer(t, func(http.ResponseWriter, *http.Request) {})
			client := newClient(t, srv)
			q := sampleQuestion(t)
			opts := tt.mutate(&q)

			res, err := Choose(context.Background(), client, q, opts...)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Choose error = %v, want %v", err, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("Choose error = %q, want it to say what to use instead (%q)", err, tt.wantMsg)
			}
			if res != nil {
				t.Fatalf("Choose returned %v alongside an error, want no result", res)
			}
			if got := hits.Load(); got != 0 {
				t.Fatalf("server received %d requests, want 0", got)
			}
		})
	}
}

func TestChooseRejectsANilClient(t *testing.T) {
	res, err := Choose(context.Background(), nil, sampleQuestion(t))

	if !errors.Is(err, ErrNilClient) || res != nil {
		t.Fatalf("Choose(nil client) = %v, %v; want ErrNilClient and no result", res, err)
	}
}

func choiceBody(choice string, confidence float64) string {
	return fmt.Sprintf(`{"model":"jev-latest","answers":{"choice":{"type":"choice","choice":%q,`+
		`"confidence":%v,"probabilities":{"ship":0.6,"hold":0.3,"kill":0.1}}},`+
		`"usage":{"input_tokens":5,"output_tokens":1}}`, choice, confidence)
}

func answerWith(t *testing.T, body string) (*jev.Client, *atomic.Int32, *[]byte) {
	t.Helper()
	var sent []byte
	srv, hits := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		sent, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(body))
	})
	return newClient(t, srv), hits, &sent
}

func TestChooseTurnsConfidenceIntoTheRightCase(t *testing.T) {
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
		{"custom thresholds hesitate", 0.6, []ChooseOption{WithThresholds(wide)}, "uncertain"},
		{"custom thresholds escalate", 0.3, []ChooseOption{WithThresholds(wide)}, "escalate"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _, _ := answerWith(t, choiceBody("ship", tt.confidence))

			res, err := Choose(context.Background(), client, sampleQuestion(t), tt.opts...)

			if err != nil {
				t.Fatalf("Choose error: %v", err)
			}
			if got := classOf(res); got != tt.want {
				t.Fatalf("confidence %v gave %q, want %q", tt.confidence, got, tt.want)
			}
			if res.Leading() != ship || res.Confidence().Float64() != tt.confidence {
				t.Errorf("result leads with %q at %v, want ship at %v",
					res.Leading(), res.Confidence().Float64(), tt.confidence)
			}
			probs := res.Probabilities()
			if len(probs) != 3 || probs[ship].Float64() != 0.6 || probs[hold].Float64() != 0.3 || probs[kill].Float64() != 0.1 {
				t.Errorf("Probabilities() = %v, want a probability for each of the three options", probs)
			}
		})
	}
}

func TestChooseSendsOneChoiceQuestionWithTheOptions(t *testing.T) {
	client, hits, sent := answerWith(t, choiceBody("ship", 0.95))

	if _, err := Choose(context.Background(), client, sampleQuestion(t)); err != nil {
		t.Fatalf("Choose error: %v", err)
	}

	var req struct {
		State     string `json:"state"`
		Questions map[string]struct {
			Type         string            `json:"type"`
			Instructions string            `json:"instructions"`
			Criteria     map[string]string `json:"criteria"`
		} `json:"questions"`
	}
	if err := json.Unmarshal(*sent, &req); err != nil {
		t.Fatalf("request body is not JSON: %v: %s", err, *sent)
	}
	q, ok := req.Questions["choice"]
	if len(req.Questions) != 1 || !ok {
		t.Fatalf("request has questions %v, want exactly one named choice", req.Questions)
	}
	if q.Type != "choice" || q.Instructions != "What should we do with this release?" {
		t.Errorf("question = %+v, want a choice question with the instructions", q)
	}
	if len(q.Criteria) != 3 || q.Criteria["ship"] != "release it" || q.Criteria["kill"] != "drop it" {
		t.Errorf("criteria = %v, want the three options with their descriptions", q.Criteria)
	}
	if req.State != "the release candidate passed every check" {
		t.Errorf("state = %q, want the question's state", req.State)
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("server hits = %d, want exactly 1", got)
	}
}

func TestChooseReturnsFailuresAsErrorsNeverResults(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantCode string
	}{
		{"unauthorized", http.StatusUnauthorized, `{"detail":{"error_type":"auth","message":"bad key"}}`, jevclient.CodeUnauthorized},
		{"validation failure", http.StatusUnprocessableEntity, `{"detail":[{"loc":["body","state"],"msg":"Field required","type":"missing"}]}`, jevclient.CodeInvalidRequest},
		{"no answer to the question", http.StatusOK, `{"model":"m","answers":{"other":{"type":"noul","noul":0.5}},"usage":{"input_tokens":1,"output_tokens":1}}`, jevclient.CodeInvalidResponse},
		{"answer names an option outside the set", http.StatusOK, choiceBody("abandon", 0.95), jevclient.CodeInvalidResponse},
		{"confidence above one", http.StatusOK, choiceBody("ship", 1.4), jevclient.CodeInvalidResponse},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			})
			client := newClient(t, srv)

			res, err := Choose(context.Background(), client, sampleQuestion(t))

			var ce *contract.Error
			if !errors.As(err, &ce) || ce.ErrorCode != tt.wantCode {
				t.Fatalf("Choose error = %v, want a %s error", err, tt.wantCode)
			}
			if res != nil {
				t.Fatalf("Choose returned %v alongside an error, want no result", res)
			}
		})
	}
}

func TestChooseReturnsCallerCancellationUnchanged(t *testing.T) {
	srv, _ := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(choiceBody("ship", 0.95)))
	})
	client := newClient(t, srv)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := Choose(ctx, client, sampleQuestion(t))

	var ce *contract.Error
	if !errors.Is(err, context.Canceled) || errors.As(err, &ce) {
		t.Fatalf("Choose error = %v, want the bare cancellation", err)
	}
	if res != nil {
		t.Fatalf("Choose returned %v alongside an error, want no result", res)
	}
}

func TestFromAnswerRejectsUnusableAnswers(t *testing.T) {
	good := func() jev.ChoiceAnswer {
		return jev.ChoiceAnswer{
			Choice:        "ship",
			Confidence:    0.95,
			Probabilities: map[string]float64{"ship": 0.6, "hold": 0.3, "kill": 0.1},
		}
	}
	tests := []struct {
		name    string
		mutate  func(a *jev.ChoiceAnswer)
		wantErr bool
	}{
		{"control is usable", func(*jev.ChoiceAnswer) {}, false},
		{"option outside the set", func(a *jev.ChoiceAnswer) { a.Choice = "abandon" }, true},
		{"empty option", func(a *jev.ChoiceAnswer) { a.Choice = "" }, true},
		{"confidence above one", func(a *jev.ChoiceAnswer) { a.Confidence = 1.4 }, true},
		{"confidence below zero", func(a *jev.ChoiceAnswer) { a.Confidence = -0.1 }, true},
		{"confidence NaN", func(a *jev.ChoiceAnswer) { a.Confidence = math.NaN() }, true},
		{"missing probability", func(a *jev.ChoiceAnswer) { delete(a.Probabilities, "kill") }, true},
		{"no probabilities", func(a *jev.ChoiceAnswer) { a.Probabilities = nil }, true},
		{"probability above one", func(a *jev.ChoiceAnswer) { a.Probabilities["hold"] = 1.2 }, true},
		{"probability NaN", func(a *jev.ChoiceAnswer) { a.Probabilities["hold"] = math.NaN() }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answer := good()
			tt.mutate(&answer)

			res, err := fromAnswer(context.Background(), answer, sampleOptions(t), DefaultThresholds())

			if !tt.wantErr {
				if err != nil || res == nil {
					t.Fatalf("fromAnswer = %v, %v; want a result", res, err)
				}
				return
			}
			var ce *contract.Error
			if !errors.As(err, &ce) || ce.ErrorCode != jevclient.CodeInvalidResponse {
				t.Fatalf("fromAnswer error = %v, want an invalid-response error", err)
			}
			if res != nil {
				t.Fatalf("fromAnswer returned %v alongside an error, want no result", res)
			}
		})
	}
}

func TestFromResponseRejectsAResponseWithoutAUsableChoiceAnswer(t *testing.T) {
	usable := jev.ChoiceAnswer{
		Choice:        "ship",
		Confidence:    0.95,
		Probabilities: map[string]float64{"ship": 0.6, "hold": 0.3, "kill": 0.1},
	}
	tests := []struct {
		name    string
		resp    *jev.Response
		wantErr bool
	}{
		{"control is usable", &jev.Response{Answers: map[string]jev.Answer{questionName: usable}}, false},
		{"nil response", nil, true},
		{"no answers", &jev.Response{}, true},
		{"only another question", &jev.Response{Answers: map[string]jev.Answer{"other": usable}}, true},
		{"answer of another type", &jev.Response{Answers: map[string]jev.Answer{questionName: jev.NoulAnswer{Noul: 0.5}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := fromResponse(context.Background(), tt.resp, sampleOptions(t), DefaultThresholds())

			if !tt.wantErr {
				if err != nil || res == nil {
					t.Fatalf("fromResponse = %v, %v; want a result", res, err)
				}
				return
			}
			var ce *contract.Error
			if !errors.As(err, &ce) || ce.ErrorCode != jevclient.CodeInvalidResponse {
				t.Fatalf("fromResponse error = %v, want an invalid-response error", err)
			}
			if res != nil {
				t.Fatalf("fromResponse returned %v alongside an error, want no result", res)
			}
		})
	}
}
