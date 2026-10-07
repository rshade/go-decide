package clefclient

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/kataras/jev"
	"github.com/rshade/ax-go/contract"
)

func requireLiveEnv(t *testing.T) {
	t.Helper()
	if os.Getenv("CLOUDFLARE_AUTH_TOKEN") == "" || os.Getenv("CLOUDFLARE_ACCOUNT_ID") == "" {
		t.Skip("CLOUDFLARE_AUTH_TOKEN or CLOUDFLARE_ACCOUNT_ID not set")
	}
}

func TestLiveNoulRoundTrip(t *testing.T) {
	requireLiveEnv(t)
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
}

func TestLiveBadTokenIsUnauthorized(t *testing.T) {
	requireLiveEnv(t)
	t.Setenv("CLOUDFLARE_AUTH_TOKEN", "not-a-real-token")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}
	_, err = client.SystemOne(ctx, urgentRequest())
	var ce *contract.Error
	if !errors.As(Classify(ctx, err), &ce) || ce.ErrorCode != CodeUnauthorized {
		t.Fatalf("Classify() = %v, want %s", Classify(ctx, err), CodeUnauthorized)
	}
}
