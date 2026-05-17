package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps the SQLite archive.
type Store struct {
	db *sql.DB
}

// Open opens the archive and applies migrations.
func Open(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("sqlite", buildDSN(path))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), `ROLLBACK`)
		}
	}()
	current, err := currentVersion(ctx, conn)
	if err != nil {
		return err
	}
	for i, mig := range migrations {
		v := i + 1
		if v <= current {
			continue
		}
		if _, err := conn.ExecContext(ctx, mig); err != nil {
			return fmt.Errorf("migration v%d: %w", v, err)
		}
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *Store) currentVersion(ctx context.Context) (int, error) {
	return currentVersion(ctx, s.db)
}

type versionQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func currentVersion(ctx context.Context, q versionQuerier) (int, error) {
	var v sql.NullInt64
	err := q.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_version`).Scan(&v)
	if err == nil && v.Valid {
		return int(v.Int64), nil
	}
	if err != nil && strings.Contains(err.Error(), "no such table") {
		return 0, nil
	}
	return 0, err
}

func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	return s.currentVersion(ctx)
}

func buildDSN(path string) string {
	q := url.Values{}
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "synchronous(NORMAL)")
	q.Add("_pragma", "foreign_keys(ON)")
	q.Add("_pragma", "busy_timeout(5000)")
	return "file:" + path + "?" + q.Encode()
}

// RawPayload is a raw HTTP response archived for durability.
type RawPayload struct {
	Method      string
	Path        string
	Query       string
	DataType    string
	WindowStart string
	WindowEnd   string
	StatusCode  int
	Headers     http.Header
	Body        []byte
	FetchedAt   time.Time
}

// SaveRawPayload archives a raw response and updates rate-limit state.
func (s *Store) SaveRawPayload(ctx context.Context, p RawPayload) error {
	headers, err := json.Marshal(p.Headers)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(p.Body)
	now := p.FetchedAt.UnixMilli()
	if now <= 0 {
		now = time.Now().UnixMilli()
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO raw_payloads
			(method, path, query, data_type, window_start, window_end, status_code, headers_json, body_json, body_sha256, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Method, p.Path, p.Query, p.DataType, p.WindowStart, p.WindowEnd, p.StatusCode,
		string(headers), string(p.Body), hex.EncodeToString(sum[:]), now,
	)
	if err != nil {
		return err
	}
	_, _ = s.db.ExecContext(ctx, `
		UPDATE sync_meta
		   SET last_raw_payload_at = MAX(last_raw_payload_at, ?)
		 WHERE id = 1`, now)
	return s.UpdateRateLimit(ctx, p.Headers)
}

// TouchRawActivity records that a remote response was received without
// retaining the bulky response body.
func (s *Store) TouchRawActivity(ctx context.Context, fetchedAt time.Time) error {
	ms := fetchedAt.UnixMilli()
	if ms <= 0 {
		ms = time.Now().UnixMilli()
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE sync_meta
		   SET last_raw_payload_at = MAX(last_raw_payload_at, ?)
		 WHERE id = 1`, ms)
	return err
}

// PruneRawPayloads removes archived raw responses. Queryable data_points and
// rollup_points are left intact.
func (s *Store) PruneRawPayloads(ctx context.Context, olderThan time.Time) (int64, error) {
	var res sql.Result
	var err error
	if olderThan.IsZero() {
		res, err = s.db.ExecContext(ctx, `DELETE FROM raw_payloads`)
	} else {
		res, err = s.db.ExecContext(ctx, `DELETE FROM raw_payloads WHERE fetched_at < ?`, olderThan.UnixMilli())
	}
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// Vacuum compacts the SQLite database after pruning.
func (s *Store) Vacuum(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `VACUUM`)
	return err
}

// UpdateRateLimit stores latest rate-limit headers plus a local request count.
func (s *Store) UpdateRateLimit(ctx context.Context, h http.Header) error {
	now := time.Now().UnixMilli()
	_, err := s.db.ExecContext(ctx, `
		UPDATE rate_limits
		   SET limit_value = ?,
		       remaining = ?,
		       reset_value = ?,
		       local_count = local_count + 1,
		       updated_at = ?
		 WHERE id = 1`,
		firstHeader(h, "X-RateLimit-Limit"),
		firstHeader(h, "X-RateLimit-Remaining"),
		firstHeader(h, "X-RateLimit-Reset"),
		now,
	)
	return err
}

func firstHeader(h http.Header, key string) string {
	if v := h.Get(key); v != "" {
		return v
	}
	return ""
}

// Account is locally cached identity/profile/settings.
type Account struct {
	HealthUserID string `json:"health_user_id"`
	LegacyUserID string `json:"legacy_user_id"`
	ProfileJSON  string `json:"profile_json"`
	SettingsJSON string `json:"settings_json"`
	UpdatedAt    int64  `json:"updated_at"`
}

func (s *Store) SaveIdentity(ctx context.Context, body []byte) error {
	var obj struct {
		LegacyUserID string `json:"legacyUserId"`
		HealthUserID string `json:"healthUserId"`
	}
	_ = json.Unmarshal(body, &obj)
	_, err := s.db.ExecContext(ctx, `
		UPDATE account
		   SET health_user_id = COALESCE(NULLIF(?, ''), health_user_id),
		       legacy_user_id = COALESCE(NULLIF(?, ''), legacy_user_id),
		       updated_at = ?
		 WHERE id = 1`, obj.HealthUserID, obj.LegacyUserID, time.Now().UnixMilli())
	return err
}

func (s *Store) SaveProfile(ctx context.Context, body []byte) error {
	_, err := s.db.ExecContext(ctx, `UPDATE account SET profile_json = ?, updated_at = ? WHERE id = 1`, string(body), time.Now().UnixMilli())
	return err
}

func (s *Store) SaveSettings(ctx context.Context, body []byte) error {
	_, err := s.db.ExecContext(ctx, `UPDATE account SET settings_json = ?, updated_at = ? WHERE id = 1`, string(body), time.Now().UnixMilli())
	return err
}

func (s *Store) Account(ctx context.Context) (Account, error) {
	var a Account
	err := s.db.QueryRowContext(ctx, `
		SELECT health_user_id, legacy_user_id, profile_json, settings_json, updated_at
		  FROM account WHERE id = 1`).Scan(&a.HealthUserID, &a.LegacyUserID, &a.ProfileJSON, &a.SettingsJSON, &a.UpdatedAt)
	return a, err
}

// SyncMeta is global freshness state.
type SyncMeta struct {
	LastSuccessAt    int64  `json:"last_success_at"`
	LastAttemptAt    int64  `json:"last_attempt_at"`
	LastErrorAt      int64  `json:"last_error_at"`
	LastError        string `json:"last_error"`
	LastRawPayloadAt int64  `json:"last_raw_payload_at"`
	LastDataPointAt  int64  `json:"last_data_point_at"`
}

func (s *Store) SyncMeta(ctx context.Context) (SyncMeta, error) {
	var m SyncMeta
	err := s.db.QueryRowContext(ctx, `
		SELECT last_success_at, last_attempt_at, last_error_at, last_error, last_raw_payload_at, last_data_point_at
		  FROM sync_meta WHERE id = 1`).Scan(&m.LastSuccessAt, &m.LastAttemptAt, &m.LastErrorAt, &m.LastError, &m.LastRawPayloadAt, &m.LastDataPointAt)
	return m, err
}

func (s *Store) MarkSyncAttempt(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sync_meta SET last_attempt_at = ? WHERE id = 1`, time.Now().UnixMilli())
	return err
}

func (s *Store) MarkSyncSuccess(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sync_meta SET last_success_at = ?, last_error = '' WHERE id = 1`, time.Now().UnixMilli())
	return err
}

func (s *Store) MarkSyncError(ctx context.Context, errText string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sync_meta SET last_error_at = ?, last_error = ? WHERE id = 1`, time.Now().UnixMilli(), errText)
	return err
}

func (s *Store) MarkDataTypeSync(ctx context.Context, dataType, start, end string, points int, syncErr error) error {
	now := time.Now().UnixMilli()
	if syncErr != nil {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO sync_state (data_type, last_error_at, last_error, last_window_start, last_window_end)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(data_type) DO UPDATE SET
				last_error_at = excluded.last_error_at,
				last_error = excluded.last_error,
				last_window_start = excluded.last_window_start,
				last_window_end = excluded.last_window_end`,
			dataType, now, syncErr.Error(), start, end)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sync_state (data_type, last_success_at, last_error, last_window_start, last_window_end, points_seen)
		VALUES (?, ?, '', ?, ?, ?)
		ON CONFLICT(data_type) DO UPDATE SET
			last_success_at = excluded.last_success_at,
			last_error = '',
			last_window_start = excluded.last_window_start,
			last_window_end = excluded.last_window_end,
			points_seen = sync_state.points_seen + excluded.points_seen`,
		dataType, now, start, end, points)
	return err
}

// Count returns a table count.
func (s *Store) Count(ctx context.Context, table string) (int, error) {
	switch table {
	case "raw_payloads", "data_points", "rollup_points", "sync_state":
	default:
		return 0, fmt.Errorf("unsupported table %q", table)
	}
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&n)
	return n, err
}
