# Changelog

## v1.0.4 - 2026-05-17

### Changed

- Updated public positioning to `Google Health (Fitbit) Local Archive`.
- Clarified that `ghcli` uses the Google Health API for the same underlying
  health history and archives Google Health API data for Fitbit/Google health
  accounts.
- Removed public wording that implies a separate Fitbit-to-Google-Health data
  movement path or refers to the older Fitbit API surface.
- Updated bundled skill install metadata to
  `github.com/fdsouvenir/ghcli@v1.0.4`.

## v1.0.3 - 2026-05-17

### Changed

- Embedded `v1.0.3` as the default CLI version so `go install
  github.com/fdsouvenir/ghcli@v1.0.3` reports a tagged version without custom
  linker flags.
- Changed empty JSON query results from `null` to `[]` for easier agent
  handling.
- Serialized SQLite schema migrations with `BEGIN IMMEDIATE` so concurrent
  first-run commands do not race while creating a new archive.
- Updated bundled skill install metadata to
  `github.com/fdsouvenir/ghcli@v1.0.3`.

## v1.0.2 - 2026-05-17

### Changed

- Restored `metadata.openclaw.requires.bins: ["ghcli"]` in the bundled skill so
  ClawHub security analysis and runtime metadata match the skill's actual CLI
  usage.
- Updated install metadata and docs to point at
  `github.com/fdsouvenir/ghcli@v1.0.2`.
- Moved fresh-install expectations into README/release notes: install `ghcli`
  first, then authenticate and sync before asking archive questions.

## v1.0.1 - 2026-05-17

### Changed

- Updated the bundled OpenClaw skill frontmatter to use single-line JSON
  metadata for ClawHub compatibility.
- Changed the skill installer metadata to install
  `github.com/fdsouvenir/ghcli@v1.0.1`.
- Removed the hard `requires.bins` gate so the skill can still explain how to
  install `ghcli` when the binary is missing.
- Added fresh-install diagnostics for missing `ghcli`, missing OAuth state,
  stale archives, and empty archives.
- Clarified that setup, login, sync, and export commands require explicit user
  intent, while normal health questions stay local and read-only.

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

- `ghcli` uses the Google Health API for the same underlying health history.
- Default tests avoid invented API fixtures; live API tests are opt-in with
  `GHCLI_LIVE_TESTS=1`.
