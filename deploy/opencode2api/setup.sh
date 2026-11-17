#!/bin/sh
# Generates config.local.json from config.example.json with a random server
# key and prints the next steps. Run once; re-running will not overwrite an
# existing config.local.json unless --force is passed.
set -eu

DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$DIR"

FORCE=0
[ "${1:-}" = "--force" ] && FORCE=1

if [ -f config.local.json ] && [ "$FORCE" -eq 0 ]; then
  echo "config.local.json already exists. Use --force to regenerate it." >&2
  exit 1
fi

if command -v openssl >/dev/null 2>&1; then
  SERVER_KEY="oc2api-$(openssl rand -hex 24)"
elif command -v python3 >/dev/null 2>&1; then
  SERVER_KEY="oc2api-$(python3 -c 'import secrets; print(secrets.token_hex(24))')"
else
  SERVER_KEY="oc2api-$(tr -dc 'a-f0-9' < /dev/urandom | head -c 48)"
fi

sed -e "s/REPLACE_WITH_RANDOM_SERVER_KEY/$SERVER_KEY/" config.example.json > config.local.json
chmod 600 config.local.json

cat <<EOF
config.local.json generated (mode 600).

Next steps:
  1. Deploy opencode2api (download a release binary or: go build -o opencode2api ./cmd/opencode2api from https://github.com/jasonxu114514/opencode2api).
     The container option is documented in README.md.
  2. Start it:        ./opencode2api -config /path/to/config.local.json
  3. Verify:          curl -s http://127.0.0.1:8091/healthz
  4. Wire the channel in the admin UI:
       - App Channels -> OpenCode Anon -> Base URL:  http://127.0.0.1:8091
       - App Channels -> OpenCode Anon -> API key:   $SERVER_KEY
       - App Channels -> OpenCode Anon -> enable the channel and paste the
         /v1/models list of the gateway as the model list.

The generated server key is ALSO written to config.local.json only; keep the
file private, it is git-ignored. Do not paste the key into chat logs.
EOF
