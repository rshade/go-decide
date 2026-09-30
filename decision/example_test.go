package decision_test

import (
	"context"
	"fmt"
	"log"

	"github.com/rshade/jev-decide/decision"
	"github.com/rshade/jev-decide/jevclient"
)

type Action string

const (
	Ship Action = "ship"
	Hold Action = "hold"
)

func ExampleMatch() {
	client, err := jevclient.NewClient()
	if err != nil {
		log.Fatal(err)
	}
	options, err := decision.NewOptions(map[Action]string{
		Ship: "The build is safe to release now.",
		Hold: "The build needs more work before release.",
	})
	if err != nil {
		log.Fatal(err)
	}

	result, err := decision.Choose(context.Background(), client, decision.Question[Action]{
		State:        "The build passed all 412 tests and the security scan found nothing.",
		Instructions: "Is this build ready to release?",
		Options:      options,
	})
	if err != nil {
		log.Fatal(err)
	}

	next := decision.Match(result,
		func(d decision.Decided[Action]) string { return "go ahead with " + string(d.Choice()) },
		func(u decision.Uncertain[Action]) string { return "ask a person; Jev leans " + string(u.Leading()) },
		func(e decision.Escalate[Action]) string { return "run the decide debate" },
	)
	fmt.Println(next)
}

func ExampleNewThresholds() {
	floor, _ := decision.NewProbability(0.4)
	confident, _ := decision.NewProbability(0.8)

	th, err := decision.NewThresholds(floor, confident)
	fmt.Println(th.Floor().Float64(), th.Confident().Float64(), err)

	_, err = decision.NewThresholds(confident, floor)
	fmt.Println(err)
	// Output:
	// 0.4 0.8 <nil>
	// decision: invalid thresholds: floor 0.8 must be below confident level 0.4
}
