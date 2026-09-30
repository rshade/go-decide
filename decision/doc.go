// Package decision turns a Jev choice answer into a result that cannot be acted
// on without handling low confidence.
//
// [Choose] asks one choice question over a typed set of options and returns a
// [Result]: [Decided], [Uncertain] or [Escalate], chosen by comparing the
// answer's confidence with two [Thresholds]. Only a [Decided] result exposes a
// choice. [Uncertain] is for human review and [Escalate] is for the full
// debate; both expose only the leading option.
//
// Decided does not mean approved. Jev's confidence is not calibrated, so a high
// confidence means the answer is clear enough to skip the debate and nothing
// more. Deciding whether to act remains the caller's job.
//
// The default thresholds are placeholders until they are tuned on real
// decisions. Failures are errors, never results.
package decision
