# Logging

Canonical provider logging and log-masking guidance. Use the framework's logging package, `tflog`, not `slog`, `log`, or `fmt.Println`. This is a genuine "something special" case, and the reason is structural: a provider is a subprocess, not the program the user runs. Use `secrets-and-sensitive-data.md` as the canonical reference for secret state exposure.

## Why not slog

A provider runs as a separate process that Core launches over gRPC via `go-plugin`. Anything written to stdout/stderr does not reach the user's terminal like a normal program; the plugin protocol uses those streams, so stray writes can corrupt the channel or vanish. Logs must travel back to Core through the plugin's structured logging mechanism, and `tflog` is the framework's interface to exactly that: it hands log lines to `go-plugin`'s logger, which Core surfaces according to the user's `TF_LOG` settings. `slog` knows nothing about that pipe. The issue is the destination, not `slog` itself; `tflog` is a thin structured API deliberately shaped like `slog` but wired to the right place.

## Basics

The `ctx` passed into every resource method already carries the logger.

```go
import "github.com/hashicorp/terraform-plugin-log/tflog"

func (r *widgetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
    tflog.Debug(ctx, "creating widget", map[string]any{"name": plan.Name.ValueString()})
    // ...
    tflog.Trace(ctx, "widget created", map[string]any{"id": created.ID})
}
```

- Levels: `Trace`, `Debug`, `Info`, `Warn`, `Error`, surfaced via `TF_LOG` (`TF_LOG=DEBUG`), with `TF_LOG_PROVIDER` to scope verbosity to providers.
- Fields are a `map[string]any`, the main ergonomic difference from `slog`.
- Enrich the context so later logs carry fields automatically: `ctx = tflog.SetField(ctx, "widget_id", id)`.
- Define a named **subsystem** with its own level via `tflog.NewSubsystem` for a distinct layer such as the API client.

## The two things that matter for correctness

**Never log secrets.** `tflog` does not redact anything by default, and `Sensitive: true` on a schema attribute does not propagate into log calls. A debug line that dumps a token defeats the sensitivity marking entirely. Register masking on the context, and still avoid passing a raw secret as a field in the first place (defense in depth):

```go
ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "token", "password")
// or MaskAllFieldValuesRegexes / MaskMessageRegexes for pattern-based masking
```

**Logs are not how you talk to the user.** Logs are diagnostic output gated behind `TF_LOG`; most users never see them. Anything the user needs to know (a validation problem, why an apply failed, a deprecation) belongs in **diagnostics** (`resp.Diagnostics.AddError`/`AddWarning`), not a log line. The split: diagnostics are user-facing and always shown; `tflog` is for you and for users debugging with `TF_LOG` on.

## Existing slog-based clients

If an API client library already logs with `slog`, either let it keep its own logging and accept it will not be integrated into Terraform's log stream (fine for a library used outside Terraform too), or bridge it with a small `slog.Handler` that forwards records into `tflog`. The bridge is the clean answer when the client is provider-specific; for the provider's own code, call `tflog` directly.
