# ghcli

`ghcli` is a local-first Google Health (Fitbit) CLI and archive. It syncs
authorized Google Health API data into a durable SQLite archive, keeps OAuth
tokens in private local state, and answers queries from local data by default.

`ghcli` uses the Google Health API for the same underlying health history.

## Status

Current release: `v1.0.5`

This is the first stable public release. Google Health API availability still
depends on the account, granted scopes, connected devices, and Google-side API
behavior.

## Features

- Explicit OAuth setup using user-owned Google OAuth client credentials.
- Private local token storage with refresh-token rotation.
- Local SQLite archive under `$XDG_STATE_HOME/ghcli` or
  `~/.local/state/ghcli`.
- Read-only query commands for agents and shell workflows.
- Incremental sync and historical backfill commands.
- Systemd user timer templates for near-real-time polling within API limits.
- Raw API payload archival is opt-in with `--archive-raw`; normalized rows are
  stored by default.
- OpenClaw skill wrapper for local, read-only health data questions.

## Install

Install a tagged release:

```sh
go install github.com/fdsouvenir/ghcli@v1.0.5
```

Build from a checkout:

```sh
go build -ldflags "-X github.com/fdsouvenir/ghcli/cmd.Version=$(git describe --tags --always --dirty)" -o ghcli .
```

## Google OAuth Setup

Each user must create their own OAuth client for an installed application in
Google Cloud. Download the client JSON and provide it explicitly:

```sh
export GHCLI_GOOGLE_CREDENTIALS=/path/to/google-oauth-client.json
```

You can also pass the file per command:

```sh
ghcli --credentials /path/to/google-oauth-client.json auth setup
```

Or provide inline JSON directly:

```sh
export GHCLI_GOOGLE_CREDENTIALS_JSON="$(cat /path/to/google-oauth-client.json)"
```

For OpenClaw/ClawHub installs, store the full client JSON in
`skills.entries.google-health-local-archive.apiKey` as plaintext JSON or as an
OpenClaw SecretRef. OpenClaw resolves that value into
`GHCLI_GOOGLE_CREDENTIALS_JSON` for the skill process.

The client JSON is secret material and must not be committed or printed.

## Token Storage

User OAuth tokens are stored locally under the ghcli state directory by default:

```text
$XDG_STATE_HOME/ghcli/token.json
~/.local/state/ghcli/token.json
```

The state directory is created with private permissions and token files are
written with `0600` permissions. `auth setup` validates the configured client
credentials and token storage. `auth login` uses copy/paste OAuth by default.
Use `auth login --loopback` only when the configured redirect URI is a loopback
URL with an explicit port, for example `http://localhost:8080`.

## Quick Start

```sh
ghcli auth setup
ghcli auth login
ghcli sync once
ghcli doctor
ghcli --json daily --since 2026-05-17
ghcli --json activity --limit 50
ghcli --json sleep --since 2026-05-01
```

Backfill a historical window:

```sh
ghcli sync backfill --since 2026-05-01 --until 2026-05-17 --rollups
```

Print systemd user unit templates:

```sh
ghcli sync install-systemd
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

`ghcli` asks for the current Google Health read scopes and syncs the data types
the API exposes to the authorized account, including:

- activity: steps, distance, floors, altitude, calories, active minutes, active
  zone minutes, activity level, sedentary periods, exercise, swim data, and VO2
  max
- heart: heart rate, resting heart rate, heart-rate zones, and HRV
- oxygen and breathing: SpO2 and respiratory rate
- sleep: sleep records and sleep temperature derivations
- body: height, weight, and body fat
- nutrition: hydration logs
- account context: profile, settings, and observed device/source details

Availability depends on Google Health API access, the user's scopes, account
state, and device/app data availability.

## Raw Payloads

Raw Google Health API response bodies are not stored by default. Add
`--archive-raw` to `sync once` or `sync backfill` only when debugging an API
shape or preserving an audit copy.

Delete existing raw response bodies while keeping queryable rows:

```sh
ghcli maintenance prune-raw --vacuum
```

## OpenClaw Skill

The bundled skill is in `skills/google-health-local-archive`. It is designed for
read-only local querying through `ghcli --json --read-only` and never calls the
Google Health API directly.

The skill declares `ghcli` as a required binary for ClawHub/OpenClaw load-time
checks. Fresh installs should install the CLI first, then run `ghcli auth setup`,
`ghcli auth login`, and `ghcli sync once` before asking health-data questions.
Inside the skill, missing credentials, tokens, or synced data are handled with
read-only diagnostics and explicit next-step instructions.

OpenClaw users can configure Google OAuth client JSON with SecretRef without
ghcli knowing which vault/provider is used:

```json5
{
  skills: {
    entries: {
      "google-health-local-archive": {
        apiKey: { source: "env", provider: "default", id: "MY_GOOGLE_HEALTH_CLIENT_JSON" },
      },
    },
  },
}
```

## Tests

Default tests do not use invented API payloads.

```sh
go test ./...
```

Live API tests are gated:

```sh
GHCLI_LIVE_TESTS=1 go test ./internal/health -run Live
```

Run live tests only after `ghcli auth login` has stored a usable local token.

## Release Notes

Release notes are tracked in [CHANGELOG.md](CHANGELOG.md). The public notes for
`v1.0.5` are in [docs/release-notes/v1.0.5.md](docs/release-notes/v1.0.5.md).

## License

MIT. See [LICENSE](LICENSE).
