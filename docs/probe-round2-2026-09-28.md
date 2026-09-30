# Probe round 2: what Jev can do for FinFocus recommendations

Evidence behind `docs/spec/score-recommendations.md`. Everything runs from
`probe_experiments_test.go` and `probe_harness_test.go`:

```bash
go test -run 'TestProbe' -v -count=1
```

Responses are cached in `.probe-cache/` (gitignored) so reruns are free.
Total Jev spend for both rounds was about $0.12 (input $0.042 per Mtok).

## Method

- **Dataset:** 80 synthetic recommendations in FinFocus's `Recommendation`
  proto shape, written by a generator agent (`testdata/recommendations.json`),
  plus 40 recommendation pairs for duplicate detection
  (`testdata/rec_pairs.json`). The generator's own class labels were kept
  away from the labellers.
- **Labels:** two blind labellers (Sonnet and Opus agents) rated each record
  independently on four questions. Agreement is high, so their consensus is a
  credible yardstick: kappa 0.81 on safe-to-apply (74 records agree), kappa
  0.89 on false positive (76 agree), priority Spearman 0.91.
- **Metric:** AUC against consensus labels (1.0 is perfect ranking, 0.5 is
  chance). Model: `jev-latest`, which equals `jev-1.13.0`.
- **Caveats:** the data is synthetic and LLM-written, the labellers are LLMs,
  and n is 80. Real plugin output must be checked before trusting any number
  here as a production figure.

## Results

### What state helps (risk, false positive, worth-acting AUC)

| State sent | Risk | False positive | Act now | Tokens per rec |
| --- | --- | --- | --- | --- |
| Full recommendation | 0.91 | 0.91 | 0.92 | 916 |
| What core keeps today (id, type, description, savings) | 0.80 | 0.68 | 0.66 | 687 |
| Without tags | 0.88 | 0.87 | 0.92 | 885 |
| Without metadata | 0.91 | 0.88 | 0.88 | 878 |
| Without tags and metadata | 0.88 | 0.79 | 0.83 | 847 |
| Without utilization | 0.92 | 0.90 | 0.94 | 897 |
| Without resource id and name | 0.91 | 0.91 | 0.92 | 865 |
| Without plugin confidence and source | 0.91 | 0.88 | 0.93 | 895 |
| Minimal allowlist (description, type, provider, resource type, tags, costs, effort hours, metadata) | 0.89 | 0.90 | 0.89 | 764 |
| Prose rendering of the full record | 0.91 | 0.90 | 0.91 | 729 |

Tags and metadata carry the false-positive signal. Resource identifiers add
nothing to these three signals. The core's current reduced record loses about
0.1 to 0.26 AUC, so scoring needs the full `Recommendation`.

### Thresholds (full state, consensus labels)

| Signal | Threshold | Flagged | Precision | Recall |
| --- | --- | --- | --- | --- |
| Risk at or above | 0.2 | 54 | 0.89 | 0.87 |
| Risk at or above | 0.3 | 43 | 1.00 | 0.78 |
| Risk at or above | 0.5 | 27 | 1.00 | 0.49 |
| False positive at or above | 0.3 | 27 | 0.81 | 0.79 |
| False positive at or above | 0.6 | 12 | 1.00 | 0.43 |

Risk 0.3 and false positive 0.3 are workable review flags on this data.

### Do not use scores to auto-approve

- A gate of risk below 0.3, false positive below 0.3 and worth-acting at or
  above 0.5 approved 28 records. Only 7 were "good" by consensus (safe, not a
  false positive, priority 2 or higher), and 10 were unsafe. Precision 0.25.
  Even the strictest gate tried (0.2, 0.2, 0.5) approved 10 with 4 unsafe.
- Brier score for risk is 0.236, close to the 0.25 of an uninformative
  forecast. Jev ranks well (AUC 0.92) but its numbers are not calibrated
  probabilities on this data. Treat them as ranking scores.
- Missed-unsafe cases fell into two groups: reserved-instance and committed-use
  purchases (financial lock-in) and gray cases such as sandbox idle instances
  and unattached disks. Adding "locking in a financial commitment that is hard
  to undo" to the risk question raised risk AUC from 0.92 to 0.96.

### Wording and question design

- The first-round wording ("production workload", yes means safe) scored 0.73
  once inverted, against 0.92 for the reworded question.
- Asking four questions in one request or one per request gives the same
  results (risk 0.91 against 0.92, false positive 0.91 against 0.91, worth
  acting 0.92 against 0.93). Joint requests are cheaper.
- Priority as a four-level score with plain level names ("Ignore, Low,
  Medium, High") tracks the labellers best (Spearman 0.77 with mean labeller
  priority). A rubric with dollar thresholds scored 0.29, a choice question's
  expected value 0.60, and a worth-acting yes/no 0.67. Keep rubrics plain.
- `jev-preview`, `jev-1.13.0` and `jev-latest` gave the same AUCs today.

### Noise, batching and order

- Five repeat runs: risk AUC 0.913 to 0.921 each; averaging five runs gave
  0.92. Per-record spread across runs averaged 0.027 and peaked at 0.070. A
  dead band of 0.1 around a threshold absorbs the noise; averaging does not
  help.
- Batching many recommendations into one request works for risk and gets
  cheaper per record. The later plugin evaluation found that the priority
  score degrades when batched (Spearman about 0.77 per record, about 0.55 at
  batch sizes 5 to 25), so this table covers risk only:

  | Batch size | Risk AUC | Tokens per rec | ms per request |
  | --- | --- | --- | --- |
  | 5 | 0.94 | 478 | 246 |
  | 20 | 0.92 | 439 | 500 |
  | 40 | 0.94 | 433 | 1008 |
  | 80 | 0.95 | 429 | 1558 |

  Sixty and 80 records fit in one request (22.5k and 30k input tokens). The
  documented cap is 32k for state plus the longest question, so about 80 is
  the ceiling for records of this size.
- Order matters a little. Reversing the order of a 20-record ordered list moved
  scores slightly (Spearman 0.95 with the forward order; AUC 0.92 against
  0.93).

### Duplicates

| Identifier mode | AUC | Accuracy at 0.5 | False merges | Missed duplicates |
| --- | --- | --- | --- | --- |
| Raw identifiers | 1.00 | 40 of 40 | 0 | 0 |
| Pseudonymized id and name | 1.00 | 38 of 40 | 2 | 0 |
| Identifiers omitted | 0.87 | 34 of 40 | 6 | 0 |

Pseudonyms keep the ranking perfect; only the threshold shifts. Omitting
identifiers makes false merges likely. The pairs are synthetic and may be
easier than real cross-plugin overlap.

### Other findings

- **Thin evidence:** an "insufficient evidence" question separates the
  generator's ambiguous records with AUC 0.83 (mean 0.64 against 0.29 for
  clearly safe records). Useful as a flag, not decisive.
- **Prompt injection:** three naive payloads (an instruction appended to the
  description, one in a tag value, and one wrapped as "untrusted") changed the
  mean risk of 20 unsafe records by 0.01, 0.02 and 0.09 and priority by 0.02,
  minus 0.20 and minus 0.13. Small, but 20 records and naive payloads only;
  free-text fields must still be treated as untrusted.
- **Throughput:** 120 requests at concurrency 40 all succeeded with no 429s
  and about 21 requests per second, matching the documented 1,200 per minute.
  Latency rose from about 200 ms to about 2 s because the server queues
  requests. Concurrency of 8 to 10 is enough.
- **Latency and cost:** a single request with four questions takes about 200
  to 300 ms. At about 430 tokens per record when batched, 1,000 records cost
  about $0.02.

## What this does not tell us

- Real plugin output: descriptions from real advisors are messier than these
  generated ones, and utilization may matter more when descriptions omit it.
- Whether the labellers' judgement matches your team's. Two LLMs agreeing with
  each other is not the same as agreeing with a human FinOps owner.
- Behaviour on a real data-set of hundreds of recommendations, other clouds'
  naming and non-English text.
- Drift: `jev-latest` moves with each release. Pin a version when tuning
  thresholds.
