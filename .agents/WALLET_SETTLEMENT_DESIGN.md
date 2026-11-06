# Wallet transfer and settlement safety — work in progress

This document is an implementation checklist, not a claim that the current draft
is safe to deploy. Reseller code issuance creates transferable value; it must not
race an operation whose final wallet debit is still unresolved.

## Invariants

1. Every potentially wallet-charged operation obtains a durable, unique hold
   **before** work capable of generating a debit begins. Holds survive process
   crashes and do not expire automatically. Code issuance and hold creation
   serialize on the same user row in the primary database.
2. The final wallet delta and release of its hold commit in the same SQL
   transaction. Retrying the same operation is idempotent, including an
   ambiguous commit; a retry with a different amount fails. Token accounting,
   restricted packages, logs and customer response require separate explicit
   consistency/retry rules; a DB commit must never be inferred from a cache.
3. An operation that hands off to an asynchronous task retains the same hold
   until the task reaches terminal status **and** its wallet settlement/refund
   is durably applied. Updating a task to SUCCESS before its final debit must
   not create a gap. Failed settlement leaves a hold and a recoverable record.
4. A hold is released only when all downstream charges (including possible
   policy/violation fees) are known. A panic, machine loss, ambiguous upstream
   result or failed settlement leaves it open for reconciliation. Recovery must
   be authorized, auditable, idempotent, and must not guess that an expired
   lease means no future charge can arrive.
5. SQL is authoritative for transferable balance. Cache loss, invalidation,
   delayed cache updates and Redis failover cannot create or repay money. Batch
   mode currently fails closed for reseller issuance; do not remove that guard
   until batch deltas and settlement are accounted for durably.
6. Existing billing explicitly allows negative balance when actual cost exceeds
   precharge. A hold prevents transfers *during* a charge but does not by itself
   stop a user transferring value and then starting a new under-reserved charge.
   Avoid claiming debt/overspend prevention until the billable maximum, usage
   limits, trusted-user bypass and legacy paths have a coherent enforced policy.

## Inventory requiring integration and tests

- `service/billing.go`, `service/billing_session.go` and
  `service/funding_source.go`: precharge, reservation increases, trusted zero
  precharge, settle/refund, token/pool fallback and errors after responses.
- `controller/relay.go`, `relay/relay_task.go`, `service/task_billing.go`,
  `service/task_polling.go`: async submission, insert failure, task completion,
  timeout/failure, poll overlap, provider retries and task reconciliation.
- `service/quota.go` / `relay/realtime`: legacy `PostConsumeQuota` and live
  WebSocket per-usage billing; `service/violation_fee.go`: late penalty fees.
- `middleware/distributor.go` / `service/byog/router.go`: BYOG dispatch and its
  separate wallet debit; it must not ignore debit failures.
- Administrative/manual balance adjustments, promotions into the reseller role,
  restricted balance package insert/deduction, batch writes and multi-instance
  deployment must follow the same authority/locking rules.

## Existing foundation (not wired into the above)

`model/wallet_transfer_hold.go` provides SQL holds, user-row serialization,
range-checked atomic settlement and replay detection; reseller issuance checks
for open holds. It is deliberately not connected to a subset of live billing
paths because partial coverage could be mistaken for complete protection. The
model tests and MySQL/PostgreSQL integration tests establish only the foundation.

## Release gates

- Tests with explicit barriers for issuance versus every debit path and task
  state transition; multiple nodes and genuine Redis loss/failover.
- Kill/restart and ambiguous commit tests at every transition, including task
  handoff and recovery; exact-once accounting or explicit safe manual recovery.
- Bounds/authorization tests for maximum cost, streaming, unknown usage,
  trusted users, async jobs and direct BYOG, plus restricted-package semantics.
- Operational recovery UX/runbook and database retention/archiving that cannot
  remove idempotency records while a retry remains possible; load/performance
  assessment for SQL contention and per-request write volume.
- Device and payment-provider tests remain independent release gates.
