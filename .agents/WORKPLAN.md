# BYOG overhaul work plan (session arena/27446b23)

## A. Server (Go)
- [x] A1 Shared monotonic download-telemetry merge (model/byog.go) used by BOTH RegisterByogWorker (HTTP) and HandleHeartbeat (WS) -> fixes progress bar jumping.
- [x] A2 RegisterByogWorker: accept download_model_id/download_started_at/download_speed_bps/eta_seconds/download_error; use merge helper.
- [x] A3 Hub.Unregister: fail all PendingRequests immediately (synthetic inference_error) -> playground no longer hangs 180s when node dies.
- [x] A4 Tunnel: read deadline + pong/heartbeat_ack keepalive refresh; half-open conn cleanup.
- [x] A5 DispatchInference: first-token timeout 300s (model load on slow phones), inter-token 90s; parse error payload JSON; proper non-stream JSON error; respChan buffer 1024; skip empty final chunks in token count/SSE.
- [x] A6 handler.go: build role-tagged transcript prompt from messages for workers (mobile uses prompt) -> playground history works.
- [x] A7 ModelStore: worker disconnect during streamModelToWorker continues caching in background (do not fail job).
- [x] A8 Remove /api/byog/admin/stats endpoint + controller (BYOG admin stats deleted per request).

## B. Mobile client (Java templates + build_apk.py)
- [x] B1 WorkerService crash handler: do not swallow main-thread crashes (zombie freeze "bir daha acilmıyor"); persist crash + restart activity/kill like MainActivity handler.
- [x] B2 runWorker/onStartCommand: move heavy disk I/O (resolveModelFile import copy, GGUF validation, StatFs) OFF the main thread -> ANR/crash at pairing (Honor Pad 10).
- [x] B3 onCreate: startForeground ASAP; catch ForegroundServiceStartNotAllowedException -> stopSelf gracefully.
- [x] B4 WakeLock with timeout + renewal each tick (Honor aggressive kill).
- [x] B5 Exact-alarm permission (SCHEDULE_EXACT_ALARM/USE_EXACT_ALARM) + fallback to inexact set on SecurityException.
- [x] B6 QRScanActivity: pick back-facing camera via CameraInfo (tablets!), correct display orientation from sensor+rotation, cache preview size, reuse buffers, catch Throwable, no-camera fallback message, camera release/reopen on pause/resume.
- [x] B7 LocalNeuralEngine: serialize generations with global lock (concurrent generate = native crash), strict GGUF header validation before load (truncated file = SIGSEGV RealMe C55), bound maxTokens, explicit error when native lib missing.
- [x] B8 executeInference: only mark model installed if valid on disk; if missing -> inference_error + requeue download; stop generating if tunnel disconnected; use full transcript prompt.
- [x] B9 Download loop: move HTTP sync off the read loop (speed!); authoritative total bytes (no estimate flip-flop); monotonic per-attempt broadcasts; finalize at 100% before reset; server URL priority; speed/eta fields; report download_model_id + download_started_at in heartbeat/register/broadcasts; retry w/ resume; cap full restarts.
- [x] B10 MainActivity: safe evalJs helper wrapping all evaluateJavascript calls; keep crash-restart loop bounded.

## C. Desktop client (Go)
- [x] C1 IDM downloader: probe real size via HEAD (fixes truncated model when DB size stale), per-chunk resume+retry, streaming merge (no full-part ReadFile), identity encoding, mirror fallback.
- [x] C2 downloadModelSingle: resume via Range, retries.
- [x] C3 llama servers: stop other-model servers before starting new (RAM exhaustion -> playground dead), cap concurrent servers.

## D. Web: BYOG admin
- [x] D1 Remove stats tab + component + api fn + types; default section -> nodes (globe); sidebar url -> /byog-admin/nodes; remove i18n keys byog.admin.tab_stats & stats-tab keys. (tab_stats + common.refresh + 51 orphan stats_* keys removed from all 8 locales.)

## E. Web: Site-wide Statistics (new admin page)
- [x] E1 Backend model/site_stats.go + controller/site_stats.go + routes (admin): summary (dozens of metrics), time series w/ granularity+tz, rankings (users/models/channels/tokens/groups/ips), distributions (hourly/weekday/logtype/stream), filters: range preset+custom, include/exclude users, model, channel, group. Cross-DB portable (modulo hour buckets; no FLOOR/date fns).
- [x] E2 Frontend feature web/src/features/site-statistics + route /statistics + sidebar entry (Admin) + charts (recharts) + tables + filters + i18n (en,tr real; en fallback copies in other 6 locales via i18n:sync).
- [x] E3 routeTree.gen.ts regenerated via bun run build (TanStack Router plugin).

## F. Tests & verification
- [ ] go build ./... ; go vet touched pkgs; go test — BLOCKED: no Go toolchain in sandbox. Substituted structural checks: brace/paren balance on all 13 touched Go files, single-pass unused-import lint (0 problems), full cross-reference resolution of new/changed symbols (all resolve), zero dangling refs to deleted byog_stats symbols.
- [ ] relaykit: GOWORK=off go build ./... — BLOCKED: same (no Go toolchain); relaykit untouched by this change set.
- [x] web: bun install, bun run typecheck (clean; also fixed 8 PRE-EXISTING errors present on base c6ad556 in client-page/pair-modal/mail-dialog/stats-tab), bun run build (exit 0), bun run test (52 files / 313 tests all pass).
- [x] mobile template sanity: build_apk.py py_compile OK; all 5 embedded Java string blocks brace/paren balanced; WorkerService (555/555 braces) + LocalNeuralEngine (111/111) templates balanced. go test ./worker/mobile N/A (no Go toolchain; package is Python+Java templates only).

## G. Git
- [x] commit on arena branch, push, PR -> merge to main (gh pr merge), verify. Remote branches: only origin/main existed pre-push; after merge the arena branch remains (session is tracked by it) — no stale non-main branches to delete.

## H. Statistics and BYOG observability follow-up (session arena/7f197559)
- [x] Statistics: name-search filters, loading and retry states, concurrent visible time-series and rankings, token split and global BYOG distributions.
- [x] BYOG admin: operational health, crash report pages and detail dialog, model/download error list; new read-protected endpoints and tests.
- [x] New UI strings translated into all eight locales; locale sync run.
- [ ] Existing translation backlog remains (sync report: fr 87, ja 423, ru 422, vi 87, zh 197; additionally some existing dashboard strings are English in zh-TW). Do not treat sync as translation.
- [x] Web typecheck, build, existing suite (313 tests) and new targeted UI tests (5 tests) passed locally.
- [x] Backend Go vet, build and tests passed in GitHub CI after fixing fixture uniqueness and the quoted group query. The local sandbox has no Go toolchain.
- [ ] Real browser and all-client end-to-end run: sandbox Chromium launch failed due to missing libnspr4/libnss3; Android/desktop toolchains and devices unavailable. A 30-minute all-client live test has not been performed.
- [ ] Translation and real-runtime validation requirements remain outstanding; the user requested merging the partial work with these limitations.
- [ ] Verify picker authorization for delegated admins: user/channel search endpoints require their respective read permissions, whereas site statistics uses AdminAuth alone. Avoid broadening access without a deliberate permission decision.

## I. Statistics authorization, client attribution and translations (session arena/db0a5b41)
- [x] Register `statistics:read`, restrict site statistics and its minimal filter-option endpoints, and guard the frontend route and sidebar without granting user/channel management access.
- [x] Return API-key rankings separately from application rankings. Client names are best-effort, optional client-supplied headers; historical or unidentified requests remain unknown and headers can be spoofed.
- [x] Preserve translated, user-friendly labels in closed selects; prioritize exact usernames and numeric user IDs before the 50-result search limit.
- [x] Translate the statistics and administrator permission catalog strings across eight locales and run locale sync. Added a regression test for the covered translations; unrelated pre-existing translation debt remains.
- [x] Web typecheck, build, and complete Vitest suite passed (55 files, 328 tests), including new statistics, ranking, picker, and locale coverage.
- [ ] Full web lint is not clean (512 errors and 244 warnings across the repository); see /home/user/statistics-web-lint.log.
- [x] Go vet, build and tests passed in GitHub CI for this follow-up (run 42161791322). The local sandbox has no Go toolchain; cross-database migrations and queries still need runtime validation.
- [ ] The requested real-client, 30-minute runtime verification remains unperformed: the sandbox lacks a working browser, native client toolchains and devices. No live-client verification is claimed.

## J. BYOG and reseller follow-up (session arena/fcb68de0)
- In progress: HF parameter metadata and conservative, explicitly approximate resource estimates; no fabricated file sizes when metadata is unavailable.
- In progress: Windows worker WebView failure fallback and WebSocket write serialization. The tracked Windows installer and executable were rebuilt on GitHub Actions, and their PE subsystems are GUI (2); real Windows runtime verification is still outstanding. Existing clients will not auto-update solely from this rebuild because the shared worker version was not bumped.
- In progress: archive crash reports to the configured log directory before deleting their database rows; current node error states are not historical logs.
- In progress: remove the reseller child-domain, branding, gateway UI, API and code. Existing databases retain the legacy configuration table and any secrets. Existing child-domain payments may be disrupted; reconciliation is needed before deployment.
- Review found serious existing BYOG worker authentication gaps and reseller balance races/cache incoherence; the tunnel and telemetry endpoints trust node IDs without device authentication; concurrent reseller balance reservation/refund is unsafe, and customer-log access is too broad. They are not yet remediated. Real Android/Windows tests remain outstanding. Do not merge or delete branches while requested work is incomplete. GitHub Actions passed Go vet, build, tests, Windows cross-build, web typecheck and tests; local web production build and 330 tests passed.

## K. Reseller and BYOG security follow-up (session arena/fcb68de0)
- User explicitly requested merging PR #43 first; merged as 11f80ec68e86dab5528181e933a1d7367a046c89. The security findings and device-test caveats documented in J still apply to main.
- In progress: new reseller code creation limited to balance, with historical non-balance refund support; atomic debit/code creation and redemption refund CAS, Redis-aware reservation, and removal of broad customer log access. CI and concurrent/cache review required before release.
- BYOG worker authentication is not yet implemented. Safe migration requires authenticated registration, tunnel, crash telemetry, a credential lifecycle and compatible rebuilt clients. Existing workers cannot be assumed compatible.
- The reseller prototype now fails closed in batch-update mode because cache rehydration may see a stale database, and refuses issuance when the user holds active model-restricted quota packages (otherwise restrictions could be laundered). This is an availability limitation, not a complete quota-cache redesign.
- GitHub Actions run 42253876662 passed the initial SQLite/Redis tests, Go vet/build and the full frontend suite. Further concurrency and deployment review across all databases and cache failure modes remains required. Real Windows/Android device checks and BYOG authentication remain undone.

## L. Authenticated worker migration (session arena/0112f587)
- PR #45 is a draft and MUST NOT be merged yet. Reseller commits from #44 were cherry-picked. Balance-only creation, transaction/CAS checks and restrictive fail-closed cases are implemented but Redis/SQL failure recovery and cross-database concurrency need a full review.
- Worker credential migration is under construction: server generates a per-worker random token, stores its hash, requires it for registration refresh, tunnel, crash reporting, status, resource and inference test. Pairing codes are one-use and expire in 15 minutes. Old owner-pair endpoint was removed. The old session cannot disconnect a newer session; shared HWIDs no longer cause other users' nodes to be deleted. The separate logged-in owner paths enforce ownership.
- Desktop and Android sources were changed; desktop bridge and Android native bridge proxy status/resources without exposing the worker credential to JavaScript. Desktop watchdog and registration refresh include the credential. Android WebView TLS errors are cancelled, mixed content disabled, and socket hostname verification added. The Android WebView exposes its JavascriptInterface to arbitrary frames/navigation: navigation, frame isolation, file-origin permissions and the trust boundary still require a security redesign and device testing before shipping. Do not treat these source changes as a completed security audit.
- Current checked-in worker binaries and APK are OLD and incompatible with the new server protocol. ALL supported worker artifacts and the Windows installer/APK must be rebuilt, release versioning planned and actual installation/pairing/resource/updates tested. Android Java/template changes have not been compiled or run on an Android device. Windows GUI cross-build alone does not prove no CMD window in runtime.
- Protocol still permits HTTP, which transmits credentials in cleartext. Require HTTPS/WSS for deployment or implement a deliberate, tested local-development exception. Pairing sessions are in memory and require same-instance routing; failed registration consumes the code. Review session revocation/races during delete/ban, credential storage and redirection, and ensure secrets are not logged.
- CI run 42261419709 passed backend vet/build/tests, Windows cross-build and web typecheck/tests at commit 9fb61ee8f, including the desktop bridge regression test. This is not proof of Android compilation, real devices, MySQL/PostgreSQL or payment-provider E2E. Local web typecheck and reseller UI test passed. Full frontend lint remains pre-existing and failing.
- Previous changes remain on main from PR #43. PR #44 remains open on the older branch; do not close it or delete branches before the replacement work is completed, verified and merged.

## M. Follow-up status (supersedes provisional statements in J–L)
- BYOG registration, tunnel and worker telemetry now authenticate using random, server-hashed per-node credentials. Pairing codes are database-backed, hashed, single-use and expire after 15 minutes; failures after a code is consumed require a fresh code. Multi-instance pairing uses the shared primary database.
- Native worker connections require HTTPS except loopback development (and Android emulator 10.0.2.2). WebView navigation, frames, mixed content and secret-bearing operations are restricted. Credentials are captured with their server identity. These controls are not a substitute for real device and security testing.
- The tracked clients have been rebuilt from version 1.10.0 in Windows CI run 42265710862. The Android Java sources compiled and the APK was signed; both Windows executables have PE GUI subsystem 2. Windows/Android runtime, pairing, background service, updates and installer behavior remain untested on actual devices.
- The old repository-shared Android signing key is compromised. The server now creates an installation-specific key (or uses `BYOG_APK_SIGNING_KEY`), does not fall back to the intermediate APK, and requires a persistent shared backup for consistent updates. The old key is still in Git history and cannot be made secret again.
- Reseller balance code issuance is atomic with a SQL guarded debit, rejects batch quota mode and active model-restricted packages, and invalidates cache on failed issuance. Non-batch wallet reservations also check SQL so a stale/rehydrated cache cannot independently approve a second reservation. This does NOT make all billing paths transactionally consistent: final API settlement and other balance adjustments can be unconditional after a request is performed. Redis failover, distributed in-flight settlement, model-package concurrency, and multi-instance correctness need a comprehensive ledger/settlement redesign and testing before claiming financial invariants.
- A CI job exercises concurrent reseller issuance/refund and pairing on disposable MySQL and PostgreSQL services. CI outcome on the final commit must be checked. SQLite/miniredis coverage is not a substitute for production Redis failover tests.
- Crash report deletion archives the report data before DB deletion; broader actor/action audit coverage and failure/concurrency behavior remain review items. Existing child-panel database tables/secrets and third-party callbacks may require operational cleanup; real payment-provider flows are untested.
- Live BYOG WebSocket sessions are held in a process-local hub. Shared-database pairing does not imply cross-instance inference dispatch; deployments must direct BYOG workers and relevant requests to one coordinator until a distributed hub/failover is implemented and tested.

## N. Wallet settlement redesign (in progress; NOT release-ready)
- The relay guard, async task handoff, legacy task holds and root-only audited manual reconciliation are implemented. SQL wallet events make non-batch precharge, extra reservations, settlement/refund and task wallet adjustments replay-safe; the detailed runbook and remaining gaps are in `.agents/WALLET_SETTLEMENT_DESIGN.md`.
- This does not cover every legacy, realtime, BYOG, violation-fee or task/token accounting failure, nor does it prevent creating new unbounded API debt after balance has already been converted into transferable codes. Full fault injection, multi-instance/Redis failover, and a maximum-cost authorization design remain outstanding. Keep PR #45 draft; do not claim the financial redesign is complete or merge it until the release gates are satisfied.
- CI results apply only to the specific commits tested. Local Go is unavailable. Windows/Android real-device and payment-provider tests have not been performed.

- Follow-up: Midjourney pending-charge markers prevent new poller refunds before a charge is known; non-batch wallet refunds atomically clear the task quota, stale task updates preserve billing markers, and legacy final, BYOG, violation and Midjourney charges use hold events where present. These changes reduce specific failure windows but do not satisfy the remaining release gates above. Inspect the latest CI results, not an earlier commit.
- Realtime now joins its WebSocket readers before releasing the request hold, preserves uncertain failures for investigation, and settles cumulative turn reservations without charging the same usage twice. Direct-write limited token reservations independently check SQL instead of trusting a rehydrated cache. The unbounded-cost and complete fault-injection gates remain open.
