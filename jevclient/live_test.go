package jevclient

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/kataras/jev"
)

func TestLiveNoulRoundTrip(t *testing.T) {
	if os.Getenv("TYPESAFE_API_KEY") == "" {
		t.Skip("TYPESAFE_API_KEY not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}

	resp, err := client.SystemOne(ctx, jev.Request{
		State: "I was charged twice. Please help.",
		Questions: jev.Questions{
			"billing": jev.Noul{Instructions: "Is this message about billing?"},
		},
	})
	if err != nil {
		t.Fatalf("SystemOne() error: %v", Classify(ctx, err))
	}

	answer, ok := resp.Noul("billing")
	if !ok {
		t.Fatal("response has no noul answer for billing")
	}
	if answer.Noul < 0 || answer.Noul > 1 {
		t.Fatalf("noul = %v, want a probability from 0 to 1", answer.Noul)
	}
	if resp.RequestID == "" {
		t.Error("response has no request ID")
	}
}
