# Spike: Jev on decision-shaped input (2026-09-29)

Question: does Jev behave usefully on "choose between options" input, and does
its confidence separate clear-cut decisions from contested ones? That
separation is what a fast-path-then-escalate design for `jev-decide` needs.

Run with `go test -run TestSpike -v -count=1` (`jev_decide_spike_test.go`).

## Setup

- 40 synthetic decisions (`testdata/decisions.json`), written by a generator
  agent: infrastructure, build-vs-buy, architecture, hiring, pricing,
  compliance, tooling. Each has context, 2 to 5 checkable constraints and 2 to
  4 options with cost and time.
- Truth file (`testdata/decisions_truth.json`), kept from Jev: 20 "dominant"
  decisions where exactly one option satisfies every constraint, and 20
  "contested" ones where two or more options do and no answer is objectively
  right.
- Three questions per decision: a choice over the options, "is one option
  clearly best", and "would reasonable engineers disagree".

## Results

| Check | Result |
| --- | --- |
| Correct pick on dominant decisions | 20 of 20 |
| Mean pick confidence, dominant vs contested | 0.98 vs 0.58 |
| Detecting contested decisions (AUC) | 0.99 from 1 minus pick confidence; 0.98 from entropy or 1 minus top probability; 0.97 from "clear winner"; 0.86 from "disagreement" |
| Pick changes when option order is reversed | 0 of 20 dominant, 2 of 20 contested |
| Pick changes across identical repeats | 0 of 40 dominant, 0 of 40 contested |

Spend was under $0.01.

## Reading it honestly

- The dominant decisions are constraint-checking puzzles: the stated numbers
  eliminate every option but one. Jev is good at that, and it is a favourable
  test. Real strategic decisions have fuzzier, unstated and conflicting
  constraints.
- "Contested" here means several options satisfy the constraints. It does not
  prove Jev senses genuine expert disagreement on open-ended questions.
- The choice confidence is the useful signal, not the yes/no "disagreement"
  question, which is weaker and should not be the escalation trigger.
- Same caveats as the recommendation probes: synthetic and LLM-written data,
  n of 40, and one model version.

## Implication for jev-decide

The fast path is plausible for constraint-shaped decisions: take Jev's pick
when the choice confidence is high (the two classes separate well around
0.9 on this data, to be re-tuned), and escalate to the `decide` debate when
it is not. Before relying on that for real decisions, run the same test on
past decisions whose outcomes you know.
