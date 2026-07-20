# Concepts and Lifecycle

The execution model every provider decision flows from. Read this before implementing anything.

## What a provider is

A provider is a standalone Go binary that Terraform Core launches as a subprocess and talks to over gRPC (via `hashicorp/go-plugin`). Core owns the dependency graph, the plan/apply orchestration, and the state file. The provider owns one job: translate declarative desired state into API calls and report observed state back. The contract is the Terraform Plugin Protocol (versions 5 and 6; the framework speaks both). You implement Go interfaces; the framework handles the protocol.

## The three documents

Every resource operation works with up to three views of the same instance, all shaped by that resource's schema:

- **Config**: what the user literally wrote in HCL. May contain unknown values (unresolved references) and nulls (unset optionals).
- **Plan**: Core's proposed next state, computed from config plus prior state plus plan modifiers. Values that cannot be known until apply are marked **unknown**.
- **State**: the last observed truth, persisted after a successful apply.

Which one you read depends on the method:

| Method | Read from | Write to |
|---|---|---|
| Create | `req.Plan` | `resp.State` |
| Read | prior `req.State` | `resp.State` (refreshed) |
| Update | `req.Plan` (and prior `req.State` if needed) | `resp.State` |
| Delete | prior `req.State` | (removes on success) |

## The three value conditions

Every value is **known**, **null**, or **unknown**. Unknown means "known after apply" — a value that depends on something not yet created. The framework's wrapper types (`types.String`, `types.Int64`, etc.) exist precisely to represent this trichotomy, which is why you use them instead of Go primitives. Collapsing unknown into null or into a zero value is the single most common source of provider bugs (perpetual diffs, inconsistent-result errors). Never do it.

## The apply lifecycle

```
        ┌─────────┐   plan   ┌──────────┐  apply  ┌──────────┐
config ─┤  Core   ├─────────►│ provider ├────────►│   API    │
        │ (graph) │          │  (you)   │         └──────────┘
        └────┬────┘          └────┬─────┘
             │   prior state      │ new state
             └────────────────────┘
```

- **Create**: read Plan, call the create API, write State.
- **Read**: read prior State, refresh from the API, write State. This is drift detection. Refresh every attribute. Remove state only after a definitive not-found for the exact remote object, such as an authoritative 404 from a get-by-ID endpoint, so Core plans a recreate.
- **Update**: read Plan (and prior State if the diff matters), call the update API, write State.
- **Delete**: read prior State, call the destroy API.

## Per-instance scoping

`req.Plan`, `req.State`, and `req.Config` are scoped to a single resource **instance** (one address like `foo_widget.example` or `foo_widget.example["a"]`), not the whole run. The provider cannot see sibling resources, other types, or the global state file. The RPC for a resource operation carries only that instance's data. Core owns the full state and plan and never ships them to the provider.

Cross-resource references still work because **Core resolves them before your method runs**: it walks the graph, runs upstream resources first, and substitutes their resolved outputs (or `unknown` if not yet applied) into the downstream instance's Plan. You read an already-resolved field; you never traverse the reference. This is also why unknown values exist during plan: an upstream value not yet known is handed to you as unknown.

The only legitimately shared context is **provider configuration** (the client built in the provider's `Configure`, threaded into every resource via the resource's `Configure`) and per-instance **private state** (an opaque blob you stash on a resource and read back on its next operation, invisible to the user and not shared with other resources).

Why it is built this way: isolation (a provider cannot observe or corrupt unrelated resources), centralized orchestration (ordering, parallelism, and reference resolution stay in Core), and a small stateless-per-call protocol (which lets Core parallelize independent resources).

## How idempotency is enforced

Idempotency is **not** a property of your `Create` function and the framework does not enforce it. Core enforces it on top of **state**:

- Address in config but not in state → Create.
- Address in both → Read to refresh, then Update if config differs, else no-op.
- Address in state but not in config → Delete.

So Create runs for an address only when that address is absent from state. A second `apply` with no config change reads state, refreshes through Read, finds no diff, and plans nothing. Within one apply, Core visits each node once, so Create runs exactly once per instance.

Your responsibility is narrow but strict, because Core can only gate correctly if state reflects reality:

1. **Write the ID into state the instant the create API returns success**, before any later step that could fail. See `state-safety.md` for the detailed failure mode and repo-backed examples.
2. **Read must detect existence honestly**: refresh all attributes, and remove state only when the API result is definitive not-found for that exact object. If existence is inferred from list or search metadata, absence is not enough unless the API contract makes it authoritative for the object.

The one gap Core cannot close: if the create API succeeds but the process dies before state is durably written, the next run creates again. What closes that is below Terraform: the remote API's uniqueness constraints (a duplicate create fails with "already exists"), or idempotency tokens (AWS `ClientToken`, Stripe idempotency keys) that dedupe server-side. Rely on these for true safety; most providers quietly depend on API-side uniqueness as the real net.

State **locking** is a different concern: it prevents two concurrent runs from corrupting state. It does not make a single create idempotent. Intentional recreation (`RequiresReplace`, `terraform taint`/`-replace`) is not a violation; Core deliberately schedules one delete and one create, still gated by state.
