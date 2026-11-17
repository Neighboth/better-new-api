# OpenCode Anon sidecar (opencode2api)

The `OpenCode Anon` channel routes OpenCode free models. OpenCode's free
models are restricted to requests that look like the official OpenCode
clients — the upstream answers anything else with `429 FreeUsageLimitError`
/ `403`, no matter how the IP or ASN looks. Pointing this app directly at
`opencode.ai` can therefore never serve those models.

[`opencode2api`](https://github.com/jasonxu114514/opencode2api) is a small
Go gateway that speaks the public OpenCode inference surface *as a shaped
client* (official agent prompt, official tool vocabulary, anonymous `public`
credential, `/inference/openai/v1/chat/completions`). Deploy it as a
sidecar on the same machine and the `OpenCode Anon` channel talks to it.

Free models served this way are still rate limited and may sometimes fail.
If you need reliability, use OpenCode Go or a Zen key instead — configure
`zen_keys`/`go_keys` in `config.local.json` (they are preferred over the
anonymous lane automatically) and keep `anonymous: true` as fallback, or
switch to a paid plan.

## Verified

Configuration from `config.example.json` was exercised on a Linux server in
the same class as this one: `inkling-2-small-free` answered HTTP 200
through the gateway's `/v1/chat/completions` with the anonymous lane.
Anonymous access is upstream policy: no SLA, heavy blocking is possible.

## Files

| File | Purpose |
| --- | --- |
| `config.example.json` | Production template (anonymous, `free_only`, direct). |
| `setup.sh` | Generates `config.local.json` with a random local server key. |
| `opencode2api.service` | Example systemd unit (non-root). |

## Install (bare metal / VM, this repo's deployment model)

1. Build or download the gateway (Go 1.24+):

   ```sh
   git clone --depth 1 https://github.com/jasonxu114514/opencode2api.git
   cd opencode2api
   go build -o opencode2api ./cmd/opencode2api
   ```

   Or grab a release binary from the project's releases page.

2. Generate the config:

   ```sh
   cd deploy/opencode2api
   ./setup.sh            # writes config.local.json (mode 600)
   ```

3. Copy `opencode2api` (binary) and `config.local.json` to
   `/opt/opencode2api`, then install and start the unit:

   ```sh
   sudo cp opencode2api.service /etc/systemd/system/opencode2api.service
   sudo systemctl daemon-reload
   sudo systemctl enable --now opencode2api
   curl -s http://127.0.0.1:8091/healthz
   ```

4. Rewire the channel (Admin → Channels → `jjkk`, id 178):

   | Field | Value |
   | --- | --- |
   | Base URL | `http://127.0.0.1:8091` |
   | API key | the server key printed by `setup.sh` |
   | Models | output of `curl -s -H 'Authorization: Bearer <server key>' http://127.0.0.1:8091/v1/models` (currently the 12 free models) |
   | Status | enabled |

   Keep the channel type `OpenCode (Anon)` (or `OpenAI`-compatible), same
   model list shape as before.

## Docker Compose

A profile-tagged sidecar service is included in `docker-compose.yml`:

```sh
./deploy/opencode2api/setup.sh          # generate config.local.json first
mkdir -p deploy/opencode2api && cp deploy/opencode2api/opencode2api.service /tmp/   # not needed in docker
docker compose --profile opencode up -d opencode2api
```

The gateway listens on `127.0.0.1:8091` on the host (bound to localhost
only), and the `new-api` container reaches it on `http://opencode2api:8080`.
Point the channel Base URL there (or `http://127.0.0.1:8091` when
`new-api` runs outside Docker), and use the same generated server key.

## Notes

- The server key is **local** to the gateway — it never reaches upstream;
  upstream auth is the anonymous `public` credential or your Zen/Go keys.
- `config.local.json` must stay private and is git-ignored.
- If the anonymous lane opens a persistent 403-abuse block, add a Zen key:
  the gateway prefers keyed tiers automatically (`prefer: go`).
- For Docker secrets style setups, do not bake the generated key into the
  image; mount the file.
