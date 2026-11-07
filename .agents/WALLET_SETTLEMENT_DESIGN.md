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
  fails; closed records must not be deleted while a replay is possible. Held,
  non-batch wallet sessions read SQL balance and do not use the trusted-user
  zero-precharge shortcut. A hold
  remains open after process failure, panic or known ambiguous accounting error.
  Final event and hold closure are separate commits: a crash between them leaves
  a hold for investigation, not an automatic recovery.
- Submitted async tasks retain their request hold. Legacy in-flight tasks gain a
  terminal-processing hold. For newly marked failed, held non-batch wallet tasks
  whose submission accounting is known, the failure transition persists a refund
  intent. Recovery atomically credits wallet and token, adjusts usage counters,
  clears task quota and intent, journals the refund and closes the hold. Stale
  status updates cannot overwrite quota or clear the intent. The poller retries
  pending refunds with a rotating cursor; old failures without intent and old
  wallet-only partial refunds are deliberately not inferred. Logs remain outside
  the transaction and may be lost. A local timeout is not proof of provider
  failure: held tasks timed out locally retain their charge and hold for manual
  reconciliation, even if their quota is zero. Other task sources and successful
  settlement are not covered by this refund transaction; provider callbacks, task
  handoff and their crash/error paths still require review.
- Non-batch legacy final settlement, violation fees, direct BYOG charges and
  Midjourney submission charges now also journal their wallet delta when a
  request hold is present. For held non-batch Midjourney requests the known
  fixed price is atomically reserved from SQL wallet and API token before
  upstream submission; a shared decision event makes finalization and release
  mutually exclusive. A hold-linked task, pending marker and SQL usage counters
  finalize in the same transaction as the billable decision. Nonbillable release
  restores wallet and token and clears any linked pending task atomically.
  Ambiguous upstream errors keep the hold and reservation for review. Pollers
  skip pending tasks and failed charges retain their hold for investigation. A wallet refund consumes the persisted task
  quota, credits the user, restores token quota and adjusts SQL usage counters in
  one transaction. The poller commits `refund_pending` with a provider-confirmed
  failure status and retries marked refunds after a process restart. Stale status
  updates cannot restore quota or clear this marker. Refund logs still use a
  potentially separate log database and are not atomic with the financial commit;
  a crash can lose that log. Batch billing retains its older non-atomic behavior;
  reseller issuance is rejected in batch mode. Pre-migration failures have no
  refund-intent marker and require explicit rollout/reconciliation; no historical
  backfill is assumed. Issuance also rejects pending Midjourney charges.
- Realtime now waits for both WebSocket readers (including in-flight billing)
  before the request can release its hold, serializes writes to the client and
  reserves cumulative turn usage against the billing session instead of
  charging it a second time at final settlement. In the legacy no-session path,
  per-turn wallet adjustments have distinct hold events. Failures retain the
  hold, and uncertain final settlement is skipped for manual reconciliation.
  These controls do not cap a provider's response cost before a turn starts.
- Realtime, other unjournaled paths and every workflow above still need a full
  exact-once and error/crash review; not every wallet mutation is journaled.
- Held, non-batch paid BYOG inference snapshots its price and atomically reserves
  the maximum authorized amount from both SQL wallet and API token before worker
  dispatch. It defaults missing/unlimited `max_tokens` to 4096, rejects values
  above 32768, bounds prompt bytes, validates reported input usage, and rejects
  output chunks beyond the authorized count/byte limit before delivering them.
  Unused authorization is returned to wallet and token in one replay-safe SQL
  transaction. Unknown outcomes retain both reservation and hold. Own-node free
  inference and batch/legacy no-hold paths are not covered by this authorization.
  The worker can still lie about usage *within* the accepted bound and forge
  output chunks; this is not a trustworthy tokenizer or anti-farming proof.
  Provider rewards, logs and hold closure are separate from the balance commit.
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
   For Midjourney inspect `billing_pending`, the persisted task quota and the
   `midjourney-precharge` and `midjourney-decision` events, including both wallet
   and token deltas, before any intervention. A pending marker does not prove
   whether upstream work completed. After proving the operation stopped and
   gathering provider evidence, root can call
   `POST /api/wallet-holds/HOLD_ID/midjourney-finalize` with
   `{"user_id": USER_ID, "billable": true_or_false, "task_id": TASK_ID, "reason": "...", "confirmed": true}`.
   Billable finalization requires a matching pending task. Generic hold resolution
   rejects unfinished Midjourney reservations and wallet-only adjustments.
   Manual finalization does not guarantee a consume log; logs are separate.
   Do not clear a pending marker merely because it is old.
   For BYOG inspect both `byog-precharge` and `byog-final`, including their
   wallet and token deltas. The worker may still be running: this API does not
   verify worker termination. After proving it stopped and determining the final
   cost, root can call `POST /api/wallet-holds/HOLD_ID/byog-finalize` with
   `{"user_id": USER_ID, "actual_quota": ACTUAL, "reason": "...", "confirmed": true}`.
   Actual must be between zero and the reserved amount. The wallet and token
   settlement is replay-safe; hold closure is a separate replay-safe commit.
   Generic hold resolution rejects an unfinished BYOG reservation and does not
   allow a separate wallet-only adjustment for a BYOG hold.
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
