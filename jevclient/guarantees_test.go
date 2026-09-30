package jevclient

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/kataras/jev"
)

func TestRedirectIsNotFollowed(t *testing.T) {
	target, targetHits := newServer(t, okHandler)
	origin, _ := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusTemporaryRedirect)
	})
	setEnv(t, "test-key", origin.URL)
	c, err := NewClient(fastBackoff)
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}

	if _, err = c.SystemOne(context.Background(), safeRequest()); err == nil {
		t.Fatal("SystemOne() succeeded across a redirect, want error")
	}
	if got := targetHits.Load(); got != 0 {
		t.Fatalf("redirect target received %d requests, want 0", got)
	}
}

func TestInvalidSuccessfulResponsesYieldErrorAndNoValue(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "requested answer absent",
			body: `{"model":"m","answers":{"other":{"type":"noul","noul":0.5}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		},
		{
			name: "no answers at all",
			body: `{"model":"m","usage":{"input_tokens":1,"output_tokens":1}}`,
		},
		{
			name: "answer type differs from question",
			body: `{"model":"m","answers":{"safe":{"type":"score","score":1,"confidence":0.5,` +
				`"legend":{"0":"a"},"probabilities":{"0":1}}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		},
		{
			name: "probability above one",
			body: `{"model":"m","answers":{"safe":{"type":"noul","noul":1.4}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		},
		{
			name: "probability below zero",
			body: `{"model":"m","answers":{"safe":{"type":"noul","noul":-0.1}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		},
		{
			name: "noul field missing",
			body: `{"model":"m","answers":{"safe":{"type":"noul"}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			})
			setEnv(t, "test-key", srv.URL)
			c, err := NewClient(fastBackoff)
			if err != nil {
				t.Fatalf("NewClient() error: %v", err)
			}

			resp, err := c.SystemOne(context.Background(), safeRequest())
			if !errors.Is(err, jev.ErrResponse) {
				t.Fatalf("SystemOne() error = %v, want jev.ErrResponse", err)
			}
			if resp != nil {
				t.Fatalf("SystemOne() returned %+v alongside an error, want nil", resp)
			}
		})
	}
}

func TestConfidenceOutsideZeroToOneIsRejected(t *testing.T) {
	const usage = `,"usage":{"input_tokens":1,"output_tokens":1}}`
	choiceQ := jev.Questions{"q": jev.Choice{Criteria: map[string]any{"a": "first", "b": "second"}}}
	scoreQ := jev.Questions{"q": jev.Score{Criteria: []any{"low", "high"}}}
	choiceBody := func(confidence string) string {
		return `{"model":"m","answers":{"q":{"type":"choice","choice":"a","confidence":` + confidence +
			`,"probabilities":{"a":0.6,"b":0.4}}}` + usage
	}
	scoreBody := func(confidence string) string {
		return `{"model":"m","answers":{"q":{"type":"score","score":0.5,"confidence":` + confidence +
			`,"legend":{"0":"low","1":"high"},"probabilities":{"0":0.5,"1":0.5}}}` + usage
	}
	tests := []struct {
		name      string
		questions jev.Questions
		body      string
		wantErr   bool
	}{
		{"choice control is valid", choiceQ, choiceBody("0.6"), false},
		{"choice confidence above one", choiceQ, choiceBody("1.2"), true},
		{"choice confidence below zero", choiceQ, choiceBody("-0.1"), true},
		{"score control is valid", scoreQ, scoreBody("0.5"), false},
		{"score confidence above one", scoreQ, scoreBody("1.2"), true},
		{"score confidence below zero", scoreQ, scoreBody("-0.1"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			})
			setEnv(t, "test-key", srv.URL)
			c, err := NewClient(fastBackoff)
			if err != nil {
				t.Fatalf("NewClient() error: %v", err)
			}

			resp, err := c.SystemOne(context.Background(), jev.Request{State: "x", Questions: tt.questions})

			if !tt.wantErr {
				if err != nil || resp == nil {
					t.Fatalf("SystemOne() = %v, %v; want a valid response", resp, err)
				}
				return
			}
			if !errors.Is(err, jev.ErrResponse) {
				t.Fatalf("SystemOne() error = %v, want jev.ErrResponse", err)
			}
			if resp != nil {
				t.Fatalf("SystemOne() returned %+v alongside an error, want nil", resp)
			}
		})
	}
}
