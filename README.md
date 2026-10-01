# jev-decide

Typed decision support on top of TypeSafe AI's Jev (System One) API. The
project is in early development; see [CONTEXT.md](CONTEXT.md) for boundaries and
[ROADMAP.md](ROADMAP.md) for plans.

## Jev client

`jevclient.NewClient` returns a configured `kataras/jev` client. It is the
supported way to build one: it applies the credential, endpoint and retry
policies below, and calling `jev.New` directly bypasses them.

| Variable | Purpose |
| --- | --- |
| `TYPESAFE_API_KEY` | Bearer token from `console.typesafe.ai/keys`. Required. Read only from the environment. |
| `TYPESAFE_BASE_URL` | Optional endpoint. Must be `https`, or `http` to a loopback IP address. |
| `TYPESAFE_LOG_LEVEL` | Optional: `debug`, `info`, `warn`, `error` or `off`. Logs to stderr with the key redacted. |

Requests go to `https://api.typesafe.ai` by default. Redirects are never
followed. Only 429 and 529 are retried (at most twice, within 30 seconds,
honoring `Retry-After` up to 10 seconds);
timeouts and 5xx responses are reported, not retried. A response that omits a
requested answer or reports a probability outside 0 to 1 is an error, never a
zero value.

```go
client, err := jevclient.NewClient()
if err != nil {
    log.Fatal(err)
}

resp, err := client.SystemOne(ctx, jev.Request{
    State: "I was charged twice. Please help.",
    Questions: jev.Questions{
        "billing": jev.Noul{Instructions: "Is this message about billing?"},
    },
})
```

Keep the token in the environment or an ignored `.env`. Never commit it.

## Command line

`jev-decide ask` and `jev-decide score` put a choice or an ordered rubric to Jev
and print a versioned JSON outcome. The exit code says whether the answer can
be acted on: 0 decided, 10 uncertain, 11 escalate, and 1 to 4 for failures.

```sh
jev-decide ask --state "All 412 tests passed." --instructions "Ship it?" \
  --option ship="safe to release" --option hold="wait"
```

Input is a JSON decision spec (`--spec file` or `-`), flags, or both. See
[docs/jev-decide-cli.md](docs/jev-decide-cli.md) for the spec format, output
fields, exit codes and the versioning policy.

## Decisions

`decision.Choose` asks Jev one choice question over a typed set of options and
returns a sealed result: `Decided`, `Uncertain` or `Escalate`. Which one depends
on the answer's confidence and two thresholds.

| Result | Confidence | Meant for |
| --- | --- | --- |
| `Decided` | at or above the confident level | Acting, unless you want a second opinion |
| `Uncertain` | from the floor up to the confident level | A person |
| `Escalate` | below the floor | The full `decide` debate |

Only `Decided` has a `Choice()`. The other two expose `Leading()`, which is
context and not a decision. `Match` needs a handler for each case, so a missing
case does not compile. Failures are returned as errors, never as results.

Before any request is sent, `Choose` validates the question and returns an
error that names the field (`*decision.FieldError`, matched with `errors.As`):
a nil or empty `State`, an estimated size over the API's 32k limit for state
plus question, and more than 255 options are all rejected at no cost. Call
`Question.Validate` to check a question without a client.

`Decided` means clear enough to skip the debate, not approved: Jev's confidence
is not calibrated.

The default thresholds are a floor of 0.5 and a confident level of 0.9. Both are
placeholders until issue #6 tunes them on real decisions. On one live call, a
clear-cut "is this build ready to release?" question came back at 0.69, which
these defaults call `Uncertain`, so expect to tune them.

```go
result, err := decision.Choose(ctx, client, decision.Question[Action]{
    State:        "All 412 tests passed and the security scan is clean.",
    Instructions: "Is this build ready to release?",
    Options:      options,
})
if err != nil {
    return err
}
next := decision.Match(result,
    func(d decision.Decided[Action]) string {
        return "go ahead with " + string(d.Choice())
    },
    func(u decision.Uncertain[Action]) string { return "ask a person" },
    func(e decision.Escalate[Action]) string { return "run the decide debate" },
)
```

Scores work the same way: `decision.Rate` takes ordered `Levels` and returns a
sealed `ScoreResult` that `decision.MatchScore` consumes. Only `ScoreDecided` has
a `Level()`.
