# fbitcli

`fbitcli` is a local-first Google Health archive for Fitbit data. It uses the
Google Health API only, stores normalized records in SQLite, and exposes local
read-only query commands for agents and shell use.

The legacy Fitbit Web API is intentionally not supported.

## Status

Early implementation. The Google Health API docs currently warn that breaking
changes may occur until the end of May 2026, so Google Health wire handling is
kept behind a small client/sync layer.

## Install

```sh
go build -o fbitcli .
```

## Credentials

Development credentials are read from `./ghapi-credentials.json` by default.
You can override that with:

```sh
export FBITCLI_GOOGLE_CREDENTIALS=/path/to/google-oauth-client.json
```

OAuth tokens are stored as a KeePassXC attachment by default:

```text
vault:Services/google-health-fbitcli/token.json
```

The default vault paths match the existing OpenClaw vault convention:

```text
~/.openclaw/passwords.kdbx
~/.openclaw/vault.key
```

Override with `FBITCLI_VAULT_DB`, `FBITCLI_VAULT_KEY`,
`FBITCLI_VAULT_ENTRY`, or `FBITCLI_VAULT_ATTACHMENT`.

`auth setup` validates the local credential file and ensures the KeePassXC entry
exists. `auth login` uses copy/paste OAuth by default. Use
`auth login --loopback` only when the configured redirect URI is a loopback URL
with an explicit port, for example `http://localhost:8080`.

## Quick Start

```sh
fbitcli auth setup
fbitcli auth login
fbitcli sync once
fbitcli doctor
fbitcli --json daily --since 2026-05-17
fbitcli --json activity --limit 50
fbitcli --json sleep --since 2026-05-01
```

Backfill a historical window:

```sh
fbitcli sync backfill --since 2026-05-01 --until 2026-05-17 --rollups
```

Raw Google Health API response bodies are not stored by default. Add
`--archive-raw` to `sync once` or `sync backfill` only when debugging an API
shape or preserving an audit copy.

Delete existing raw response bodies while keeping queryable rows:

```sh
fbitcli maintenance prune-raw --vacuum
```

Print systemd user unit templates:

```sh
fbitcli sync install-systemd
```

## Commands

- `auth setup`, `auth login`, `auth status`, `auth revoke-local`
- `sync once`, `sync backfill`, `sync install-systemd`
- `maintenance prune-raw`
- `doctor`
- `daily`, `activity`, `sleep`, `heart`, `hrv`, `spo2`, `breathing`,
  `temperature`, `body`, `exercise`, `nutrition`
- `profile`, `settings`, `devices`, `export`

Query commands read only from the local SQLite archive. They do not call the
remote API.

## Data Coverage

The sync engine iterates the current Google Health data type table: steps,
distance, floors, altitude, calories, active minutes, active zone minutes,
activity level, sedentary periods, exercise, swim data, heart-rate zones, VO2
max, heart rate, resting heart rate, HRV, SpO2, respiratory rate, sleep
temperature derivations, height, weight, body fat, sleep, and hydration logs.

Availability depends on the Google Health API, granted scopes, account state,
and Fitbit device/app sync. Fitbit devices do not sync directly to this CLI;
data becomes available after the Fitbit app syncs it to Google Health.

## Tests

Default tests do not use invented API payloads.

```sh
go test ./...
```

Live API tests are gated:

```sh
FBITCLI_LIVE_TESTS=1 go test ./internal/health -run Live
```

Run live tests only after `fbitcli auth login` has stored a usable token in
KeePassXC.
