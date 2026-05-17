# ghcli Agent Notes

- This repository targets Google Health API only. Do not add legacy Fitbit Web
  API hosts, OAuth flows, scopes, or clients.
- `ghapi-credentials.json` is local secret material. Do not print, copy, or
  commit its values anywhere else.
- Default tests must not depend on invented API fixture payloads. Live Google
  Health tests are gated behind `GHCLI_LIVE_TESTS=1`.
- Query commands should remain local read-only archive reads.
