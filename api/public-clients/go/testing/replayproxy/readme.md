# SDK Replay Proxy

The replay proxy records known-good SDK traffic from the Go SDK and replays it for the TypeScript and Python SDK examples. Use it to verify that public SDKs perform the same workflow over the API while still allowing sensible transport differences such as language-specific user agents.

## What It Records

The proxy supports the traffic used by the public SDK examples:

- management-plane Connect requests
- environment ops Connect requests
- agent live-message SSE streams

Fixtures live under `api/public-clients/fixtures/sdk/<scenario>/`. Each fixture contains `manifest.json`, interaction metadata, and binary request/response bodies.

External environment and agent URLs are rewritten to `/__replay/external/<route-id>` routes. The fixture manifest stores only the stable route ID and traffic class; live route origins are kept in memory while recording and are not written to committed fixtures.

## Header Matching

Replay validates SDK identity semantically instead of matching user-agent strings byte-for-byte.

Expected SDK tokens:

- Go: `ona-go-sdk/<version> (language=go; layer=sdk)`
- TypeScript: `ona-ts-sdk/<version> (language=typescript; layer=sdk)`
- Python: `ona-python-sdk/<version> (language=python; layer=sdk)`

The proxy accepts the SDK token in either `User-Agent` or `X-Ona-User-Agent`. It ignores runtime headers such as `Authorization`, `Content-Length`, `Accept-Encoding`, trace headers, and exact user-agent values after semantic validation passes.

## Record Fixtures

Record mode requires a local dev setup and a real API key for that setup.
For the repository-backed examples, the local development organization must also have a runner that can resolve `github.com` and the recording principal must be authenticated for that host. Verify the Go example works directly against the local backend before recording; if `ParseContextURL` returns `user has no runners authenticated against this host`, fix local SCM authentication first.

```sh
cd api/public-clients/go

SSL_CERT_FILE=/tmp/local-dev-certs/tls.crt \
go run ./testing/replayproxy/cmd/replayproxy \
  -mode record \
  -listen 127.0.0.1:18080 \
  -public-url http://127.0.0.1:18080 \
  -upstream https://127.0.0.1:8080/api \
  -fixture-dir ../fixtures/sdk/start_environment_and_run_command \
  -scenario start_environment_and_run_command \
  -language go
```

In another shell, run the Go example against the proxy:

```sh
ONA_API_KEY=<local-dev-api-key> \
ONA_BASE_URL=http://127.0.0.1:18080 \
go run ./sdk/examples/start_environment_and_run_command
```

Repeat the same flow for `run_codex_agent_in_environment` with a separate fixture directory and scenario name.

## Verify Fixtures

Validate a fixture:

```sh
cd api/public-clients/go

go run ./testing/replayproxy/cmd/replayproxy \
  -validate-fixture \
  -fixture-dir ../fixtures/sdk/start_environment_and_run_command
```

Run all Go, TypeScript, and Python replay examples:

```sh
api/public-clients/scripts/replay-sdk-examples.sh
```

The replay script builds the TypeScript SDK, compiles the Python SDK, validates each fixture, starts the proxy in replay mode, and runs each public SDK example against it with a dummy API key.

## Fixture Rules

- Record fixtures only from the Go SDK against local dev.
- Do not record production traffic.
- Do not commit fixtures containing real API keys, bearer tokens, environment access tokens, or conversation tokens.
- Re-record fixtures only after understanding whether a mismatch is fixture drift, an SDK bug, or an intentional API behavior change.
