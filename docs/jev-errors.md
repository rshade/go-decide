# Jev error codes

`jevclient.Classify(ctx, err)` turns a failure from a Jev call into an
`ax-go` `contract.Error`. The code set is closed, so callers and agents can
switch on `error_code` without parsing message text.

A cancellation or deadline set by the caller's own context is returned
unchanged, not wrapped, so `errors.Is(err, context.Canceled)` still works.

## Codes

| Code | Cause | Exit | `retryable` |
| --- | --- | --- | --- |
| `jev.unauthorized` | HTTP 401 | 4 | `false` |
| `jev.invalid_request` | HTTP 422, or a request rejected before sending | 2 | `false` |
| `jev.rate_limited` | HTTP 429 | 3 | `true` |
| `jev.overloaded` | HTTP 529 | 3 | `true` |
| `jev.status` | Any other non-2xx status | 1 | unset |
| `jev.timeout` | Per-attempt timeout | 3 | `true` |
| `jev.connection` | Connection failure | 3 | `true` |
| `jev.invalid_response` | A 2xx body that breaks the contract | 1 | unset |
| `jev.internal` | Anything else | 1 | unset |

Exit codes are the `ax-go` constants: 1 internal, 2 validation, 3 network,
4 auth. `retryable` is a tri-state: unset means unspecified, and is not the
same as `false`.

`jev.invalid_response` covers a missing answer, an answer of the wrong type,
and a probability or confidence outside 0 to 1. It is never turned into a zero
value.

Timeouts and connection failures are marked retryable because re-running the
command is safe, but the client itself does not retry them: the server may
already have processed, and billed, the request. Only 429 and 529 are retried
inside the client.

## Fields

| Field | Set when |
| --- | --- |
| `error_code` | Always, one of the codes above. |
| `message` | Always. Taken from the response `detail`. |
| `context.status` | The failure was an HTTP response. |
| `context.request_id` | The response had `x-typesafe-request-id`. |
| `context.error_type` | `detail` was an object with `error_type`. |
| `retryable` | The class has a retry answer (see the table). |
| `retry_after_seconds` | Retryable, and the server sent `Retry-After` or `Retry-After-Ms`. Rounded up to whole seconds. |

The API key is replaced with `[REDACTED]` in `message` and every string
in `context`, including when the server echoes it back.

The response `detail` has two shapes. An object
`{"error_type", "message"}` gives both `error_type` and `message`. A list of
`{loc, msg, type}` entries, used for 422, gives a message that names each
field, such as `state: Field required`, and no `error_type`. A body that is
not JSON gives a truncated message and no `error_type`.

## Examples

`trace_id`, `tool`, `version` and `schema_version` are filled in by the CLI and
left out below. Messages for the transport codes come from the client library
and vary with the underlying error, so treat the text as illustrative and
switch on `error_code`.

### `jev.unauthorized`

```json
{
  "error_code": "jev.unauthorized",
  "message": "Invalid API key",
  "context": {"request_id": "req_123", "status": 401},
  "retryable": false
}
```

### `jev.invalid_request`

```json
{
  "error_code": "jev.invalid_request",
  "message": "state: Field required",
  "context": {"status": 422},
  "retryable": false
}
```

### `jev.rate_limited`

```json
{
  "error_code": "jev.rate_limited",
  "message": "slow down",
  "context": {"error_type": "rate_limit", "status": 429},
  "retryable": true,
  "retry_after_seconds": 3
}
```

### `jev.overloaded`

```json
{
  "error_code": "jev.overloaded",
  "message": "The service is overloaded",
  "context": {"status": 529},
  "retryable": true
}
```

### `jev.status`

```json
{
  "error_code": "jev.status",
  "message": "upstream unavailable",
  "context": {"status": 502}
}
```

### `jev.timeout`

```json
{
  "error_code": "jev.timeout",
  "message": "jev: POST https://api.typesafe.ai/v1/systemone: timed out after 10s",
  "retryable": true
}
```

### `jev.connection`

```json
{
  "error_code": "jev.connection",
  "message": "jev: POST https://api.typesafe.ai/v1/systemone: ... connection refused",
  "retryable": true
}
```

### `jev.invalid_response`

```json
{
  "error_code": "jev.invalid_response",
  "message": "jev: invalid response: answers.safe: answer is missing"
}
```

### `jev.internal`

```json
{
  "error_code": "jev.internal",
  "message": "unexpected failure"
}
```
