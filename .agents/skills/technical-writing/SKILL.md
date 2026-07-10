---
name: technical-writing
description: Write, edit, or review Terraform provider documentation and examples for terraform-provider-ona. Use for README updates, generated provider docs, examples, release docs, local dev-loop docs, import workflows, and developer-facing prose.
---

# Technical Writing

Write docs that help Terraform users act correctly. Prefer concise, copyable examples over broad explanation.

## Workflow

1. Identify the page type: tutorial, how-to, reference, or explanation. Do not mix types unless the file already does.
2. Verify claims against repository source before writing:
   - Provider schema and registration: `internal/provider/**`.
   - API wrapper behavior: `internal/client/**`.
   - Import helper behavior: `scripts/**`.
   - Example discovery rules: `examples/README.md`.
   - Release process: `docs/release.md`.
3. Update the source docs or examples, not generated docs alone, when generation owns the output.
4. Run `make fmt` on changed HCL examples.
5. Run `make generate` when provider docs or examples should be regenerated.

## Terraform Provider Rules

- Use provider source address `gitpod-io/ona` unless release docs specifically need `registry.terraform.io/gitpod-io/ona`.
- Use placeholders such as `<organization-id>`, `<runner-id>`, and `<api-token>` instead of real IDs or tokens.
- Keep examples minimal, valid, and copyable. Show a minimal working configuration first.
- For imports, prefer Terraform import blocks and include the validation path: generate config, apply imports, then run a no-op `terraform plan`.
- State deletion caveats plainly. Removing a managed resource from configuration can delete the remote object; removing it from state stops management without deleting it.
- Do not claim API behavior that is not visible in provider code, examples, release docs, or Ona docs cited by the user.
- Keep service-account token guidance aligned with the README: bootstrap or rotate service-account tokens with a human/admin token.

## Style

- Put the action or answer in the first paragraph.
- Use imperative steps for procedures.
- Keep paragraphs to one to three sentences.
- Use fenced code blocks with language identifiers.
- Mark values readers must replace.
- Prefer exact commands over vague instructions.
- Avoid hype, superlatives, and unexplained acronyms.

## Checks

- All commands, flags, paths, resource names, and provider addresses exist or are intentionally illustrative.
- Internal Markdown links and anchors resolve.
- Examples use files the docs generator reads: `provider/provider.tf`, `data-sources/<name>/data-source.tf`, and `resources/<name>/resource.tf`.
- Generated docs are not edited as the only source of a behavior change when schema/examples should generate them.

## Done Criteria

Writing is done when the relevant claims are checked against source, examples are formatted and use placeholders, generation has run when needed, and any credentialed or live validation that was skipped is called out.

## When Stuck

- If docs and code disagree, treat code as source of truth and call out the drift.
- If an example cannot be tested without credentials, state the untested credentialed step and still validate syntax/format where possible.
- If the audience is unclear, write for a Terraform practitioner managing Ona resources from code.
