package decision

import (
	"context"
	"errors"
	"github.com/kataras/jev"
	"github.com/rshade/ax-go/contract"
	"net/http"
	"testing"
)

var errTagged = errors.New("tagged by the custom classifier")

func tagging(seen *[]error) func(context.Context, error) error {
	return func(_ context.Context, err error) error {
		*seen = append(*seen, err)
		return errTagged
	}
}

func TestWithClassifierSeesFailedCallsAndInvalidAnswers(t *testing.T) {
	failing := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) }
	wrongOption := func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"model":"m","answers":{"choice":{"type":"choice","choice":"ship","confidence":0.9,"probabilities":{"ship":1}}},"usage":{}}`))
	}
	tests := []struct {
		name    string
		handler http.HandlerFunc
		call    func(ctx context.Context, srv *serverClient, opt ChooseOption) error
	}{
		{"choose failed call", failing, func(ctx context.Context, s *serverClient, opt ChooseOption) error {
			_, err := Choose(ctx, s.client, sampleQuestion(t), opt)
			return err
		}},
		{"rate failed call", failing, func(ctx context.Context, s *serverClient, opt ChooseOption) error {
			_, err := Rate(ctx, s.client, sampleRateQuestion(t), opt)
			return err
		}},
		{"choose answer with missing probabilities", wrongOption, func(ctx context.Context, s *serverClient, opt ChooseOption) error {
			_, err := Choose(ctx, s.client, sampleQuestion(t), opt)
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := newServer(t, tt.handler)
			var seen []error
			err := tt.call(context.Background(), &serverClient{client: newClient(t, srv)}, WithClassifier(tagging(&seen)))
			if !errors.Is(err, errTagged) {
				t.Fatalf("err = %v, want the custom classifier's error", err)
			}
			if len(seen) != 1 {
				t.Errorf("classifier ran %d times, want once", len(seen))
			}
		})
	}
}

func TestChooseWithoutAClassifierStillUsesJevCodes(t *testing.T) {
	srv, _ := newServer(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
	_, err := Choose(context.Background(), newClient(t, srv), sampleQuestion(t))
	if err == nil || !containsCode(err, "jev.unauthorized") {
		t.Fatalf("err = %v, want jev.unauthorized", err)
	}
}

type serverClient struct{ client *jev.Client }

func containsCode(err error, code string) bool {
	var ce *contract.Error
	return errors.As(err, &ce) && ce.ErrorCode == code
}
