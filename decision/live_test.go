package decision

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rshade/jev-decide/jevclient"
)

func TestLiveChoose(t *testing.T) {
	if os.Getenv("TYPESAFE_API_KEY") == "" {
		t.Skip("TYPESAFE_API_KEY not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := jevclient.NewClient()
	if err != nil {
		t.Fatalf("jevclient.NewClient() error: %v", err)
	}
	options, err := NewOptions(map[action]string{
		ship: "The build is safe to release now.",
		hold: "The build needs more work before release.",
	})
	if err != nil {
		t.Fatalf("NewOptions error: %v", err)
	}

	res, err := Choose(ctx, client, Question[action]{
		State:        "The build passed all 412 tests and the security scan found nothing.",
		Instructions: "Is this build ready to release?",
		Options:      options,
	})
	if err != nil {
		t.Fatalf("Choose error: %v", err)
	}

	if !options.Contains(res.Leading()) {
		t.Errorf("Leading() = %q, want one of the options", res.Leading())
	}
	if !res.Confidence().Valid() {
		t.Error("Confidence() is not valid")
	}
	if got := len(res.Probabilities()); got != 2 {
		t.Errorf("Probabilities() has %d entries, want 2", got)
	}
	t.Logf("live result: %v", res)
}
