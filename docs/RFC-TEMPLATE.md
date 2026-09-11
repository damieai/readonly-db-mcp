# RFC-NNNN: Title

- Status: Draft
- Authors: TBD
- Created: YYYY-MM-DD
- Depends on: None

Delete instructional text when completing an RFC. Sections marked required must
not be omitted.

## Summary

Describe the capability, the user-visible outcome, and the smallest design that
delivers it.

## Context and problem

Explain the current behavior, evidence for the problem, affected operators and
agents, and why an RFC-level change is needed.

## Goals

- List measurable outcomes.

## Non-goals

- List deliberately excluded behavior.

## Proposed design

Describe public contracts, execution flow, state ownership, failure behavior,
and engine- or driver-specific decisions.

## Security and trust boundaries — required

Document identities, privileges, target and object scope, untrusted inputs,
network destinations, read-only enforcement, secret handling, and fail-closed
behavior. Include hostile or ambiguous cases and the layer that rejects each
one.

## Latency, admission, and pool budget — required

Start from the
[common agent workload contract](ARCHITECTURE.md#agent-workload-latency-budget).
Do not copy a short timeout from a driver example without reconciling it with
the full MCP request budget.

| Boundary | Default | Configurable maximum | Hard ceiling | Budget includes | Cancellation and recovery |
| --- | ---: | ---: | ---: | --- | --- |
| Whole MCP request | | | | | |
| Admission queue | | | | | |
| Batch | | | | | |
| Connection acquisition | | | | | |
| Connect / discovery | | | | | |
| Driver read / write | | | | | |
| Server statement / lock | | | | | |
| Cleanup / pool drain | | | | | |

Answer all of the following:

- Does agent reasoning between calls hold any permit, transaction, connection,
  or topology lease? It normally must not.
- Does the request context cover admission, preflight, transaction setup,
  execution, result collection, and cleanup?
- Is a batch deadline shared by the whole batch or renewed per operation?
- Can any driver default, server setting, retry, or pool wait expire before the
  advertised request deadline?
- Does each configured pool limit map to a real hard limit in the selected
  driver version?
- What happens to server work and the leased connection after cancellation?
- How are metadata and maintenance prevented from starving interactive work,
  and vice versa?
- What measurements justify any value tighter than the common profile, and how
  can an operator restore the common budget?

## Resource and response budget — required

List concurrency, queue length, row, cell, response-byte, parameter, parser,
cache, memory forecast, retry, and topology limits. Explain truncation versus
failure behavior and any interaction between these limits.

## Compatibility and migration

Describe configuration defaults, upgrades, rollback, version gates, and changes
to existing tool behavior. Treat a tighter timeout or pool limit as a breaking
availability change.

## Observability

Specify content-free metrics and audit outcomes for queueing, execution,
timeouts, cancellation, truncation, pool waits, connection reuse, and recovery.

## Test and acceptance plan — required

Include, as applicable:

- configuration tests for the 5-minute default, 10-minute maximum, and compiled
  ceilings;
- long-running success below the deadline and cancellation at the deadline;
- server-side cancellation plus permit and pool recovery;
- concurrent interactive, metadata, batch, and maintenance saturation;
- whole-batch deadline and partial-result behavior;
- driver pool hard-limit verification;
- slow connect, lock, read, write, discovery, cleanup, and topology cases;
- race, soak, memory, and response-budget tests; and
- hostile inputs proving that relaxed resource limits do not weaken read-only
  enforcement.

## Rollout and operations

Define deployment order, capacity checks, dashboards, alerts, rollback signals,
and operator overrides. Prefer a reporting replica for workloads that may use
the full request budget.

## Alternatives considered

Record rejected designs and why they do not satisfy the security, compatibility,
latency, or operational requirements.

## Completion criteria

List objective implementation, documentation, test, and rollout exit gates.
