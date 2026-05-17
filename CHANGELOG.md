# Changelog

## v1.0.0 - 2026-05-17

Initial stable public release.

### Added

- Google Health API OAuth flow with local Google OAuth client credentials.
- KeePassXC-backed token storage and refresh-token rotation.
- SQLite archive under `$XDG_STATE_HOME/ghcli` or `~/.local/state/ghcli`.
- Incremental sync, historical backfill, and systemd user timer templates.
- Local read-only query commands for daily, activity, sleep, heart, HRV, SpO2,
  breathing, temperature, body, exercise, nutrition, profile, settings, and
  devices.
- `doctor` command for auth, token, archive, sync freshness, and API health.
- Opt-in raw response archival via `--archive-raw`.
- `maintenance prune-raw` for removing stored raw payload bodies.
- OpenClaw skill wrapper for read-only local archive questions.

### Notes

- `ghcli` targets the Google Health API only. The legacy Fitbit Web API is
  intentionally unsupported.
- Default tests avoid invented API fixtures; live API tests are opt-in with
  `GHCLI_LIVE_TESTS=1`.
