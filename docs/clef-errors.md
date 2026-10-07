# clef error codes

`clefclient.Classify(ctx, err)` turns a failure from a Cloudflare clef call
into an `ax-go` `contract.Error`. The code set is closed, so callers and
agents can switch on `error_code` without parsing message text. It mirrors
`docs/jev-errors.md`, with the prefix `clef.` and the same exit codes.

A cancellation or deadline set by the caller's own context is returned
unchanged, not wrapped, so `errors.Is(err, context.Canceled)` still works.

## Codes

| Code | Cause | Exit | `retryable` |
| --- | --- | --- | --- |
| `clef.unauthorized` | HTTP 401 or 403 | 4 | `false` |
| `clef.invalid_request` | HTTP 400 or 422, or a request rejected before sending | 2 | `false` |
| `clef.rate_limited` | HTTP 429 | 3 | `true` |
| `clef.status` | Any other non-2xx status, or a 200 whose envelope says `success: false` | 1 | unset |
| `clef.timeout` | Per-attempt timeout | 3 | `true` |
| `clef.connection` | Connection failure | 3 | `true` |
| `clef.invalid_response` | A 200 body that breaks the contract | 1 | unset |
| `clef.internal` | Anything else | 1 | unset |

Exit codes are the `ax-go` constants: 1 internal, 2 validation, 3 network,
4 auth. `retryable` is a tri-state: unset means unspecified, and is not the
same as `false`.

Only 429 is retried inside the client, at most twice. Timeouts and connection
failures are marked retryable because re-running the command is safe, but the
client does not retry them: the request may already have been billed.

`clef.invalid_response` covers a missing answer, an answer of the wrong type,
and a probability or confidence outside 0 to 1. It is never turned into a zero
value.

## Fields

| Field | Set when |
| --- | --- |
| `error_code` | Always, one of the codes above. |
| `message` | Always. The first Cloudflare `errors` message, else the response text. |
| `context.status` | The failure was an HTTP response. |
| `context.cloudflare_code` | The body had an `errors` list. The code of its first entry. |
| `context.request_id` | The response had `cf-ai-req-id`. Errors raised before the model runs, such as authentication, do not. |
| `retryable` | The class has a retry answer (see the table). |
| `retry_after_seconds` | Retryable, and the server sent `Retry-After`. Rounded up to whole seconds. |

The token is replaced with `[REDACTED]` in `message` and every string in
`context`, including when the server echoes it back.

## Observed Cloudflare failures

Recorded against the live API on 2026-10-06:

| Request | Status | Cloudflare code and message |
| --- | --- | --- |
| Missing `model`, `state` or `questions` | 400 | 5006, `AiError: Bad input: Error: required properties at '/' are 'model,state,questions'` |
| Invalid token | 401 | 10000, `Authentication error` |
| Valid token, account the token cannot use | 401 | 10000, `Authentication error` |

A bad token and a wrong account look the same, so `clef.unauthorized` cannot
say which one to fix. The 403, 429 and 5xx mappings follow Cloudflare's
conventions and have not been provoked live.

## Examples

`trace_id`, `tool`, `version` and `schema_version` are filled in by the CLI and
left out below. Messages for the transport codes come from the client library
and vary with the underlying error, so treat the text as illustrative and
switch on `error_code`.

### `clef.unauthorized`

```json
{
  "error_code": "clef.unauthorized",
  "message": "Authentication error",
  "context": {"cloudflare_code": 10000, "status": 401},
  "retryable": false
}
```

### `clef.invalid_request`

```json
{
  "error_code": "clef.invalid_request",
  "message": "AiError: Bad input: required properties are missing",
  "context": {"cloudflare_code": 5006, "request_id": "233d618f", "status": 400},
  "retryable": false
}
```

### `clef.rate_limited`

```json
{
  "error_code": "clef.rate_limited",
  "message": "slow down",
  "context": {"cloudflare_code": 2001, "status": 429},
  "retryable": true,
  "retry_after_seconds": 3
}
```

### `clef.status`

```json
{
  "error_code": "clef.status",
  "message": "upstream unavailable",
  "context": {"cloudflare_code": 3000, "status": 500}
}
```

### `clef.timeout`

```json
{
  "error_code": "clef.timeout",
  "message": "jev: POST https://api.cloudflare.com/...: timed out after 10s",
  "retryable": true
}
```

### `clef.connection`

```json
{
  "error_code": "clef.connection",
  "message": "jev: POST https://api.cloudflare.com/...: connection refused",
  "retryable": true
}
```

### `clef.invalid_response`

```json
{
  "error_code": "clef.invalid_response",
  "message": "jev: response is missing the answer for \"urgent\""
}
```

### `clef.internal`

```json
{
  "error_code": "clef.internal",
  "message": "unexpected failure"
}
```
