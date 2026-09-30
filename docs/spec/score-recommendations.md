# Spec proposal: ScoreRecommendations

Status: draft for review. Not filed in `finfocus-spec` yet.

Companion files: `docs/spec/recommendation_scoring.proto` (draft wire
format, lints and builds with `buf` against `finfocus-spec` v0.6.2) and
`docs/probe-round2-2026-09-28.md` (the evidence for every number below).

## Summary

Add an optional service, `RecommendationScorerService`, with one RPC,
`ScoreRecommendations`. Core sends it the recommendations that cost-source
plugins already returned and gets back per-recommendation signals: risk,
probability the resource is in that state on purpose, whether it is worth
acting on, priority, thin evidence, and duplicate grouping. A scorer plugin
owns any model, API key and network access, so core stays free of external
clients as the constitution requires.

The first scorer is a Jev-backed plugin. The contract is model-neutral so a
rules engine, a local model or another hosted model can implement it.

## Problem

- Core generates no recommendations; plugins do. Core then only sorts by
  savings and sums them. It has no notion of risk, no way to spot a resource
  that looks idle on purpose, and no dedupe across plugins.
- What core does keep is thin. `convertProtoRecommendation` drops priority,
  confidence, tags, utilization, metadata and cost detail before anything is
  shown or ranked.
- Hand-written rules for these judgements are brittle and provider-specific.

## Goals

1. Give hosts a stable way to ask "how should I treat these recommendations?"
   without knowing which model answers.
2. Keep network access, credentials and model choice inside plugins.
3. Make the privacy cost explicit and controllable.
4. State honestly how far the scores can be trusted.

## Non-goals

- Producing, editing or applying recommendations, or generating text.
- Approving anything. A score is never permission to act.
- Calibrated probabilities. Signals are ranking scores unless a scorer says
  otherwise.
- Streaming, persistence or storing scores in the spec layer.

## Evidence that shaped the design

From round 2 probes on 80 synthetic recommendations with two independent
blind labellers (labeller agreement kappa 0.81 to 0.89):

| Finding | Design consequence |
| --- | --- |
| Full record scores AUC 0.91, 0.91, 0.92 (risk, false positive, worth acting). Core's reduced record scores 0.80, 0.68, 0.66. | The request carries full `Recommendation` messages. |
| Resource id and name add nothing to those signals. Pseudonymized ids keep duplicate ranking perfect (AUC 1.00); omitting them drops it to 0.87. | `IdentifierMode`, default pseudonymized. |
| Risk 0.3 flags unsafe records with precision 1.00, recall 0.78. The auto-approve gate had precision 0.25 and approved 10 unsafe records of 28. | Scores are review flags. The spec forbids using them as the sole gate. |
| Brier score for risk 0.236, near uninformative, though AUC is 0.92. | `ScoreCalibration` defaults to ranking-only. |
| Per-record noise averages 0.027, peaks at 0.07; averaging five runs does not help. | Hosts apply a dead band of 0.1 around thresholds. No repeat-and-average in the contract. |
| Batches of 5 to 80 keep risk and false-positive ranking at lower cost per record, but priority ranking drops (Spearman about 0.77 per record, about 0.55 batched, measured in the plugin work). Latency is about 20 ms per record. | Batch RPC with `max_batch_size`; scorers may compute individual signals per record; reference batch default 25. |
| Questions in one request or separate requests give the same results. | One backend call per batch is fine. |
| Financial lock-in cases were the main missed-unsafe class until the risk question named them. | `SCORE_SIGNAL_RISK` covers financial commitments as well as downtime. |
| Naive prompt injection barely moved scores (at most 0.09 on risk). | Still required: free text is untrusted. |

## Design

### Placement and discovery

- New file `scoring.proto` with `RecommendationScorerService`, following the
  precedent of `AllocatorService`, `UsageSourceService` and
  `SupplementalDatasetService`. A scorer is not a cost source, so the RPC does
  not belong on `CostSourceService`.
- New capability `PLUGIN_CAPABILITY_RECOMMENDATION_SCORING = 18` (17 is taken
  by `INVOICE_DATA` on main). Hosts check it before calling. A plugin
  without it is never called.
- Additive change, so a minor version (v0.7.0). No existing message changes.

### Request

- `recommendations`: complete `Recommendation` messages, 1 to
  `max_batch_size`, distinct ids. Correlation uses `Recommendation.id`.
- `signals`: optional subset. Empty means everything supported.
- `identifier_mode`: how the host already treated `resource.id` and
  `resource.name`:
  - `PSEUDONYMIZED` (default): opaque tokens, stable within one request.
  - `RAW`: cloud identifiers, allowed only when the operator opts in.
  - `OMITTED`: removed. Duplicate grouping becomes unreliable and the scorer
    returns no `duplicate_group_id`.

The host, not the scorer, performs the transformation, so identifiers never
reach the scorer's backend in the default mode.

### Response

- `results[i]` answers `recommendations[i]`, echoing `recommendation_id`. Each
  is either `RecommendationScores` or a `ResourceError`, the same per-item
  pattern as `BatchCost`. A failure on one entry never fails the batch.
- `max_batch_size` and `supported_signals` let the host adapt without a
  separate discovery call.
- `scorer` names the implementation, model, calibration claim and the backend
  request id.

### Signals

All numeric signals are optional doubles so a scorer may implement a subset.

| Signal | Range | Meaning |
| --- | --- | --- |
| `risk` | 0 to 1 | Applying it could hurt users, lose data, regress performance or lock in an unrecoverable commitment. High is risky. |
| `false_positive` | 0 to 1 | The resource is probably in the flagged state on purpose. High means the recommendation is likely wrong for it. |
| `worth_acting` | 0 to 1 | Worth an engineer's time now, weighing saving, risk and effort. |
| `priority` | 0 to 3 | Expected priority: ignore, low, medium, high. Fractions fall between. |
| `insufficient_evidence` | 0 to 1 | The record is too thin or contradictory to judge. |
| `duplicate_group_id` | string | Same non-empty value for recommendations that duplicate each other. Valid within one response only. |

Priority is a continuous score, not the existing `RecommendationPriority`
enum, to avoid false precision. Hosts map it to the enum with their own
thresholds if they need to.

### Errors

Whole-call failures use gRPC codes: `INVALID_ARGUMENT` (empty request, over
`max_batch_size`, duplicate ids, unsupported signal), `UNIMPLEMENTED`,
`UNAUTHENTICATED`, `PERMISSION_DENIED`, `RESOURCE_EXHAUSTED`, `UNAVAILABLE`.
Per-entry failures use `ResourceError`. An empty request is invalid, unlike
`BatchCost`, because there is nothing to score.

### Trust rules (normative)

1. A score MUST NOT be the only gate for an irreversible action. Hosts use
   scores to route work to human review, sort and filter.
2. Signals are ranking scores unless `scorer.calibration` is
   `SCORE_CALIBRATION_PROBABILITY`. Thresholds are chosen against labelled
   data for the deployment.
3. Every free-text field in a recommendation (description, reasoning, tags,
   metadata) is untrusted input. Scorers MUST NOT let it change their
   behaviour beyond the score, MUST cap field lengths, and SHOULD delimit it
   from instructions.
4. Scorers that send data off-host MUST document what leaves the host and
   honour `identifier_mode`.

### Threshold guidance (non-normative)

From the probes, on this dataset only, with a dead band of 0.1:

- Risk at or above 0.3 goes to review (precision 1.00, recall 0.78).
- False positive at or above 0.3 goes to "probably intentional" review
  (precision 0.81, recall 0.79).
- Thresholds should be re-tuned on real data and re-tuned when the model
  version changes.

## Host requirements (finfocus-core, outside this spec)

The spec is only useful if core changes too. Each item is a separate issue.

1. Keep the full recommendation through the engine. Today
   `convertProtoRecommendation` drops most fields; the scorer needs them.
2. Fix the recommendation cache key. `recommendations/multi/{sorted resource
   types}` ignores resource ids and contents, so scores would leak across
   stacks. Key scores by a hash of the recommendation content plus scorer name
   and model version.
3. Add a scoring step after the plugin fetch (`engine.go` around the
   `GetRecommendationsForResources` loops), gated by the capability and an
   opt-in config key. Off by default.
4. Batch by `max_batch_size`, run up to 8 batches concurrently, apply a
   per-call timeout, and degrade to unscored output when the scorer is
   unavailable.
5. Apply `identifier_mode` before sending. Pseudonyms come from an HMAC with a
   per-request key; a normalised resource id (ARN or cloud id) is used so the
   same resource matches across plugins.
6. Surface the signals: `--sort risk`, filters such as `risk<=0.3`, table and
   JSON columns, and a "needs review" marker at the configured threshold.
   Never auto-dismiss from a score.
7. Expose sensitive-data controls: opt-in per scorer, an allowlist of fields
   (minimal allowlist scored 0.89 to 0.90 against 0.91 to 0.92 for the full
   record), and a dry-run that prints exactly what would be sent.

## Reference scorer: finfocus-plugin-jev

Non-normative, and the basis for the first implementation.

- Go plugin that implements `RecommendationScorerService` using the Jev API
  (`POST /v1/systemone`, bearer key from `TYPESAFE_API_KEY`).
- The spec surface of the API is two paths, so a thin client is enough.
  `kataras/jev` covers everything needed today; generating from
  `https://api.typesafe.ai/openapi.json` is possible but the discriminated
  unions need hand-tuning.
- One backend request per batch of up to 25 recommendations, all requested
  signals as named questions, model pinned (for example `jev-1.13.0`),
  concurrency 8 to 10, retries on 429 and 529 honouring `retry-after`.
- Per-item question names are `<signal>:<recommendation id>`, and the state is
  an ordered list of recommendations. Reversing list order shifted scores
  slightly in the probe, so keep a stable order.
- Duplicate grouping: block by pseudonymized resource id, then ask a pairwise
  yes/no within each block, then take connected components at a chosen
  threshold. Pairwise results were perfect on the synthetic pairs.
- Reference wording (revised from the probes):

  | Signal | Question | Type |
  | --- | --- | --- |
  | `risk` | Would applying this recommendation as described carry a real risk of downtime, data loss, performance regression, or locking in a financial commitment that is hard to undo, judging by the resource's environment, tags and usage in the data? | yes/no |
  | `false_positive` | Is this recommendation probably inappropriate for this resource because the data suggests it looks idle or oversized on purpose (standby, seasonal or batch, compliance retention, deliberate headroom)? | yes/no |
  | `worth_acting` | Is this recommendation worth an engineer's time this week, weighing the monthly saving against the risk and effort? | yes/no |
  | `priority` | How much priority should this cost recommendation get, weighing saving, risk and whether the evidence supports it? Levels: Ignore, Low, Medium, High. | score |
  | `insufficient_evidence` | Is the evidence in this record too thin or contradictory to tell whether the recommendation is correct? | yes/no |

- Sets `scorer.calibration` to `SCORE_CALIBRATION_RANKING_ONLY`.

## SDK and repository deliverables

Following how `finfocus-spec` added the allocator and usage services:

1. `proto/finfocus/v1/scoring.proto` (from the draft) and the new enum value
   in `enums.proto`; regenerate `sdk/go/proto` and the TypeScript bindings.
2. Go SDK: an optional provider interface, capability auto-discovery,
   `optionalServices` wiring, `maxValidCapability` bump and the related
   capability-list and bounds tests.
3. Connect handler and health checker registration for the new service.
4. Validators in `sdk/go/testing` (index alignment, unique ids, ranges,
   batch limits), a reference mock, and a `RunScorerConformance` runner.
5. `docs/recommendation-scoring.md` in the style of `docs/allocator.md`,
   including the data-handling section.
6. spec-kit feature under `specs/`, conventional commits, no hand-edited
   changelog.

## Evaluation protocol

Conformance can only check structure. Quality needs a shared yardstick:

- Contribute the labelled dataset (`testdata/recommendations.json`, two
  independent label files, and the pair file) under the SDK's testdata.
- Report AUC per signal, labeller kappa, Brier score for any scorer that
  claims calibration, and duplicate accuracy at the operating threshold.
- Replace the synthetic set with sanitised real recommendations before any
  threshold is published as guidance.

## Rollout

1. Approve the contract and open the `finfocus-spec` issue.
2. Land the spec change and SDK support (v0.7.0).
3. Core: retain full recommendations, fix the cache key, add the opt-in
   scoring step and CLI surface.
4. Ship `finfocus-plugin-jev`, then run the evaluation on real data.
5. Tune thresholds, then consider a second scorer to validate that the
   contract really is model-neutral.

## Open questions

1. New service (proposed) or an RPC on `CostSourceService` next to
   `BatchCost`? The proposal follows the allocator precedent.
2. Split `risk` into operational and financial lock-in signals? One signal
   scored AUC 0.96 once the wording covered both; two signals would let hosts
   treat commitments differently.
3. Should the host or the scorer group duplicates? Host-side grouping keeps the
   contract smaller but needs an LLM-free method for cross-plugin matching.
4. Where do scores live in `finfocus-core` output: new columns, or inside
   `Recommendation.metadata`?
5. Is `max_batch_size` capped by the SDK (100 like `BatchCost`) or by token
   budget? The Jev limit is about 80 records of this size.
6. Who owns the evaluation dataset and its refresh?
7. Data terms: TypeSafe's Master Customer Agreement and data-processing
   addendum have not been read for third-party plugin use, and zero retention
   is documented as an enterprise-plan feature.
