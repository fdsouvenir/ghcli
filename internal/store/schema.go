package store

const schemaVersion = 2

var migrations = []string{
	`
	CREATE TABLE schema_version (
		version INTEGER PRIMARY KEY
	);

	CREATE TABLE account (
		id              INTEGER PRIMARY KEY CHECK (id = 1),
		health_user_id  TEXT NOT NULL DEFAULT '',
		legacy_user_id  TEXT NOT NULL DEFAULT '',
		profile_json    TEXT NOT NULL DEFAULT '{}',
		settings_json   TEXT NOT NULL DEFAULT '{}',
		updated_at      INTEGER NOT NULL DEFAULT 0
	) STRICT;
	INSERT INTO account (id) VALUES (1);

	CREATE TABLE raw_payloads (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		method       TEXT NOT NULL,
		path         TEXT NOT NULL,
		query        TEXT NOT NULL DEFAULT '',
		data_type    TEXT NOT NULL DEFAULT '',
		window_start TEXT NOT NULL DEFAULT '',
		window_end   TEXT NOT NULL DEFAULT '',
		status_code  INTEGER NOT NULL,
		headers_json TEXT NOT NULL DEFAULT '{}',
		body_json    TEXT NOT NULL,
		body_sha256  TEXT NOT NULL,
		fetched_at   INTEGER NOT NULL,
		UNIQUE(method, path, query, body_sha256)
	) STRICT;

	CREATE INDEX ix_raw_payloads_data_type_fetched
		ON raw_payloads(data_type, fetched_at DESC);

	CREATE TABLE data_points (
		name          TEXT PRIMARY KEY,
		data_type     TEXT NOT NULL,
		record_kind   TEXT NOT NULL DEFAULT '',
		start_time    TEXT NOT NULL DEFAULT '',
		end_time      TEXT NOT NULL DEFAULT '',
		civil_date    TEXT NOT NULL DEFAULT '',
		source_json   TEXT NOT NULL DEFAULT '{}',
		value_json    TEXT NOT NULL,
		raw_json      TEXT NOT NULL,
		first_seen_at INTEGER NOT NULL,
		updated_at    INTEGER NOT NULL
	) STRICT;

	CREATE INDEX ix_data_points_type_date
		ON data_points(data_type, civil_date, start_time);
	CREATE INDEX ix_data_points_type_updated
		ON data_points(data_type, updated_at DESC);

	CREATE TABLE rollup_points (
		id            TEXT PRIMARY KEY,
		data_type     TEXT NOT NULL,
		civil_date    TEXT NOT NULL DEFAULT '',
		start_time    TEXT NOT NULL DEFAULT '',
		end_time      TEXT NOT NULL DEFAULT '',
		value_json    TEXT NOT NULL,
		raw_json      TEXT NOT NULL,
		updated_at    INTEGER NOT NULL
	) STRICT;

	CREATE INDEX ix_rollup_points_type_date
		ON rollup_points(data_type, civil_date);

	CREATE TABLE sync_state (
		data_type       TEXT PRIMARY KEY,
		last_success_at INTEGER NOT NULL DEFAULT 0,
		last_error_at   INTEGER NOT NULL DEFAULT 0,
		last_error      TEXT NOT NULL DEFAULT '',
		last_window_start TEXT NOT NULL DEFAULT '',
		last_window_end   TEXT NOT NULL DEFAULT '',
		page_token      TEXT NOT NULL DEFAULT '',
		points_seen     INTEGER NOT NULL DEFAULT 0
	) STRICT;

	CREATE TABLE sync_meta (
		id                    INTEGER PRIMARY KEY CHECK (id = 1),
		last_success_at        INTEGER NOT NULL DEFAULT 0,
		last_attempt_at        INTEGER NOT NULL DEFAULT 0,
		last_error_at          INTEGER NOT NULL DEFAULT 0,
		last_error             TEXT NOT NULL DEFAULT '',
		last_raw_payload_at    INTEGER NOT NULL DEFAULT 0,
		last_data_point_at     INTEGER NOT NULL DEFAULT 0
	) STRICT;
	INSERT INTO sync_meta (id) VALUES (1);

	CREATE TABLE rate_limits (
		id             INTEGER PRIMARY KEY CHECK (id = 1),
		limit_value    TEXT NOT NULL DEFAULT '',
		remaining      TEXT NOT NULL DEFAULT '',
		reset_value    TEXT NOT NULL DEFAULT '',
		local_count    INTEGER NOT NULL DEFAULT 0,
		updated_at     INTEGER NOT NULL DEFAULT 0
	) STRICT;
	INSERT INTO rate_limits (id) VALUES (1);

	INSERT INTO schema_version (version) VALUES (1);
	`,
	`
	DELETE FROM rollup_points
	 WHERE rowid NOT IN (
		SELECT MAX(rowid)
		  FROM rollup_points
		 GROUP BY data_type, civil_date
	 );

	CREATE UNIQUE INDEX ux_rollup_points_type_date
		ON rollup_points(data_type, civil_date);

	INSERT INTO schema_version (version) VALUES (2);
	`,
}
