package eval_test

import (
	"fmt"
	"log"

	"github.com/rshade/go-decide/decision"
	"github.com/rshade/go-decide/eval"
)

func ExampleCompute() {
	label := func(class eval.Class, pick string, confidence float64, correct string) eval.Result {
		p, err := decision.NewProbability(confidence)
		if err != nil {
			log.Fatal(err)
		}
		r, err := eval.NewResult(class, pick, p, correct)
		if err != nil {
			log.Fatal(err)
		}
		return r
	}
	results := []eval.Result{
		label(eval.Dominant, "reserved", 0.98, "reserved"),
		label(eval.Dominant, "spot", 0.93, "reserved"),
		label(eval.Contested, "warm", 0.61, ""),
	}

	report, err := eval.Compute(results, decision.DefaultThresholds())
	if err != nil {
		log.Fatal(err)
	}
	accuracy, _ := report.Accuracy.Get()
	auc, _ := report.ContestedAUC.Get()
	fmt.Printf("accuracy %.2f, contested AUC %.2f, Brier %.3f\n", accuracy, auc, report.Brier)
	fmt.Printf("decided %d, of which unsafe %d\n", report.Outcomes.Decided, report.Outcomes.DecidedUnsafe)
	// Output:
	// accuracy 0.50, contested AUC 1.00, Brier 0.412
	// decided 2, of which unsafe 1
}
