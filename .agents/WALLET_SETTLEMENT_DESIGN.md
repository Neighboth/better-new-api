# Wallet transfer and settlement — draft, not release-ready

Reseller code issuance transfers value. It must not race in-flight API charges,
and a user must not be able to transfer the balance then incur an unbounded new
bill. PR #45 is a draft: the latter invariant and full crash recovery have NOT
been established. Do not deploy this as a completed financial redesign.

## Current implementation

- Authenticated billable relay requests create a durable, non-expiring SQL hold
  before the handler. Model-list routes are exempt. Creating a hold and issuing
  a balance code serialize on the user row; issuance fails if the user has open
  holds or unfinished legacy tasks. Batch quota mode and active model-restricted
  balances fail closed for reseller issuance.
- Synchronous wallet precharge, extra reserves/rollbacks, settlement and refund
  use `wallet_hold_events`: each `(hold_id, event_key)` and its wallet delta are
  committed together. An identical replay is a no-op, a contradictory replay
  fails; closed records must not be deleted while a replay is possible. A hold
  remains open after process failure, panic or known ambiguous accounting error.
  Final event and hold closure are separate commits: a crash between them leaves
  a hold for investigation, not an automatic recovery.
- Submitted async tasks retain their request hold. Legacy in-flight tasks gain a
  terminal-processing hold. Task wallet adjustments use idempotent hold events
  in non-batch mode; terminal/refund failures retain the hold. Status changes,
  token accounting, logs and provider callbacks are not one SQL transaction.
- Legacy `PostConsumeQuota`, realtime, direct BYOG, violation fees, Midjourney
  and other paths still need a complete exact-once and error/crash review; not
  every wallet mutation is journaled. In batch mode normal billing retains its
  existing batch behavior, while reseller code issuance is rejected.
- Existing billing permits actual usage to exceed a precharge and create debt.
  A hold prevents transfer *while* work is pending, not transferring first and
  then starting a new under-reserved operation. Preventing this without breaking
  API use needs a durable maximum-cost/usage-bound authorization policy for all
  providers, streams, trusted-user bypasses and asynchronous jobs. A post hoc
  SQL nonnegative check alone would deliver unpaid work and is not a solution.

## Root recovery runbook (high-risk)

1. Keep the reseller's transfers blocked. Page through root-only
   `GET /api/wallet-holds?user_id=USER_ID&limit=50&offset=0` (limit 1–100;
   increase offset to inspect further results). Results include source,
   request/task reference and recorded SQL wallet events. Offset pagination is
   not a consistent snapshot if records change; repeat the inspection as needed.
2. Investigate the request/provider logs, task status, upstream completion and
   every related debit/refund and token adjustment. The journal covers only the
   paths listed above. An old hold is not proof the operation ended. Explicitly
   ensure it cannot still charge, refund, be retried or finish on another node.
   If uncertain, **do not resolve**; escalate for reconciliation.
3. Calculate any remaining signed SQL quota adjustment independently of already
   committed events. Only after the operation and all related work are provably
   stopped/settled, root may call `POST /api/wallet-holds/HOLD_ID/resolve` with
   `{"user_id": USER_ID, "delta": SIGNED_QUOTA_DELTA, "reason": "...", "confirmed": true}`.
   The reason must be 10–500 characters. The operator ID, note, adjustment and
   closure commit together; repeated identical resolution is idempotent.
   Incorrect resolution can release funds while a real bill is still pending.
4. Do not prune hold or event rows until a formally defined retention window
   guarantees no retry, replay or reconciliation can reference them. Back up the
   primary database; monitor open-hold age and blocked resellers. There is no
   automatic timeout, recovery worker or end-user reconciliation interface.

## Remaining release gates

- Durable authorization/usage bounds against transfer-before-new-debt; include
  trusted, streaming, unlimited token, asynchronous and legacy provider paths.
- Prove atomicity/idempotency and reconciliation for every wallet and token
  mutation, task handoff/terminal transitions, cancellation, provider failures,
  multiple pollers, cross-instance concurrency and ambiguous commits.
- Process kill/restart fault injection across each boundary, real Redis loss and
  failover, all three databases, load and SQL lock contention. CI concurrency
  tests alone are not a substitute for these checks.
- Independent real Windows/Android and payment-provider testing; BYOG live hub
  still requires a single coordinator, even with shared-database pairing.
