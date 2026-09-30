package jevclient_test

import (
	"context"
	"fmt"
	"log"

	"github.com/kataras/jev"

	"github.com/rshade/jev-decide/jevclient"
)

func ExampleNewClient() {
	client, err := jevclient.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.SystemOne(context.Background(), jev.Request{
		State:     "I was charged twice. Please help.",
		Questions: jev.Questions{"billing": jev.Noul{Instructions: "Is this message about billing?"}},
	})
	if err != nil {
		log.Fatal(err)
	}

	if answer, ok := resp.Noul("billing"); ok {
		fmt.Printf("probability of billing: %.2f\n", answer.Noul)
	}
}
