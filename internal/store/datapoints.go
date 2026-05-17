package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// DataPointRow is the query shape returned by local reads.
type DataPointRow struct {
	Name        string `json:"name"`
	DataType    string `json:"data_type"`
	RecordKind  string `json:"record_kind"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	CivilDate   string `json:"civil_date"`
	SourceJSON  string `json:"source_json,omitempty"`
	SummaryJSON string `json:"summary_json,omitempty"`
	ValueJSON   string `json:"value_json"`
	RawJSON     string `json:"raw_json,omitempty"`
	UpdatedAt   int64  `json:"updated_at"`
}

// IngestDataPoints extracts and upserts dataPoints from a Google Health list response.
func (s *Store) IngestDataPoints(ctx context.Context, dataType, recordKind string, body []byte) (int, string, error) {
	var resp struct {
		DataPoints    []map[string]any `json:"dataPoints"`
		NextPageToken string           `json:"nextPageToken"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, "", fmt.Errorf("parse dataPoints response for %s: %w", dataType, err)
	}
	now := time.Now().UnixMilli()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, "", err
	}
	count := 0
	for _, point := range resp.DataPoints {
		raw, _ := json.Marshal(point)
		name := stringValue(point["name"])
		if name == "" || strings.HasSuffix(name, "/") {
			sum := sha256.Sum256(raw)
			name = "synthetic/" + dataType + "/" + hex.EncodeToString(sum[:])
		}
		source, _ := json.Marshal(valueMap(point["dataSource"]))
		valueObj := typedValue(point, dataType)
		value, _ := json.Marshal(valueObj)
		start, end, civil := timesFromValue(valueObj)
		if civil == "" {
			civil = civilDateFromAny(valueObj["date"])
		}
		if civil == "" {
			civil = dateFromTime(start)
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO data_points
				(name, data_type, record_kind, start_time, end_time, civil_date, source_json, value_json, raw_json, first_seen_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(name) DO UPDATE SET
				data_type = excluded.data_type,
				record_kind = excluded.record_kind,
				start_time = excluded.start_time,
				end_time = excluded.end_time,
				civil_date = excluded.civil_date,
				source_json = excluded.source_json,
				value_json = excluded.value_json,
				raw_json = excluded.raw_json,
				updated_at = excluded.updated_at`,
			name, dataType, recordKind, start, end, civil, string(source), string(value), string(raw), now, now,
		)
		if err != nil {
			_ = tx.Rollback()
			return 0, "", err
		}
		count++
	}
	if count > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE sync_meta SET last_data_point_at = MAX(last_data_point_at, ?) WHERE id = 1`, now); err != nil {
			_ = tx.Rollback()
			return 0, "", err
		}
	}
	return count, resp.NextPageToken, tx.Commit()
}

// IngestRollup extracts and upserts rollupDataPoints from a dailyRollUp response.
func (s *Store) IngestRollup(ctx context.Context, dataType string, body []byte) (int, error) {
	var resp struct {
		RollupDataPoints []map[string]any `json:"rollupDataPoints"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, fmt.Errorf("parse rollupDataPoints response for %s: %w", dataType, err)
	}
	now := time.Now().UnixMilli()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	for _, point := range resp.RollupDataPoints {
		raw, _ := json.Marshal(point)
		value := typedValue(point, dataType)
		valueJSON, _ := json.Marshal(value)
		start, end, civil := timesFromValue(point)
		if civil == "" {
			civil = dateFromTime(start)
		}
		id := dataType + "/" + civil
		_, err := tx.ExecContext(ctx, `
			INSERT INTO rollup_points (id, data_type, civil_date, start_time, end_time, value_json, raw_json, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(data_type, civil_date) DO UPDATE SET
				id = excluded.id,
				value_json = excluded.value_json,
				raw_json = excluded.raw_json,
				updated_at = excluded.updated_at`,
			id, dataType, civil, start, end, string(valueJSON), string(raw), now,
		)
		if err != nil {
			_ = tx.Rollback()
			return 0, err
		}
	}
	return len(resp.RollupDataPoints), tx.Commit()
}

// QueryDataPoints returns local rows.
func (s *Store) QueryDataPoints(ctx context.Context, dataTypes []string, since, until string, limit int) ([]DataPointRow, error) {
	if limit <= 0 {
		limit = 100
	}
	where := []string{"1=1"}
	args := []any{}
	if len(dataTypes) > 0 {
		ph := make([]string, len(dataTypes))
		for i, dt := range dataTypes {
			ph[i] = "?"
			args = append(args, dt)
		}
		where = append(where, "data_type IN ("+strings.Join(ph, ",")+")")
	}
	if since != "" {
		where = append(where, "(civil_date >= ? OR start_time >= ?)")
		args = append(args, since, since)
	}
	if until != "" {
		where = append(where, "(civil_date <= ? OR start_time <= ?)")
		args = append(args, until, until)
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, `
		SELECT name, data_type, record_kind, start_time, end_time, civil_date, source_json, value_json, raw_json, updated_at
		  FROM data_points
		 WHERE `+strings.Join(where, " AND ")+`
		 ORDER BY COALESCE(NULLIF(start_time, ''), civil_date) DESC, updated_at DESC
		 LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DataPointRow
	for rows.Next() {
		var r DataPointRow
		if err := rows.Scan(&r.Name, &r.DataType, &r.RecordKind, &r.StartTime, &r.EndTime, &r.CivilDate, &r.SourceJSON, &r.ValueJSON, &r.RawJSON, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.SummaryJSON = summarizeValue(r.DataType, r.ValueJSON)
		out = append(out, r)
	}
	return out, rows.Err()
}

// QueryRollups returns local daily rollup rows as DataPointRow-compatible objects.
func (s *Store) QueryRollups(ctx context.Context, dataTypes []string, since, until string, limit int) ([]DataPointRow, error) {
	if limit <= 0 {
		limit = 100
	}
	where := []string{"1=1"}
	args := []any{}
	if len(dataTypes) > 0 {
		ph := make([]string, len(dataTypes))
		for i, dt := range dataTypes {
			ph[i] = "?"
			args = append(args, dt)
		}
		where = append(where, "data_type IN ("+strings.Join(ph, ",")+")")
	}
	if since != "" {
		where = append(where, "civil_date >= ?")
		args = append(args, since)
	}
	if until != "" {
		where = append(where, "civil_date <= ?")
		args = append(args, until)
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, data_type, start_time, end_time, civil_date, value_json, raw_json, updated_at
		  FROM rollup_points
		 WHERE `+strings.Join(where, " AND ")+`
		 ORDER BY civil_date DESC, updated_at DESC
		 LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DataPointRow
	for rows.Next() {
		var r DataPointRow
		if err := rows.Scan(&r.Name, &r.DataType, &r.StartTime, &r.EndTime, &r.CivilDate, &r.ValueJSON, &r.RawJSON, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.RecordKind = "rollup"
		r.SummaryJSON = summarizeValue(r.DataType, r.ValueJSON)
		out = append(out, r)
	}
	return out, rows.Err()
}

// QuerySources returns distinct data source JSON blobs observed in data points.
func (s *Store) QuerySources(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT source_json, COUNT(*) AS n, MAX(updated_at) AS last_seen
		  FROM data_points
		 WHERE source_json <> '{}'
		 GROUP BY source_json
		 ORDER BY last_seen DESC
		 LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var src string
		var count int
		var last int64
		if err := rows.Scan(&src, &count, &last); err != nil {
			return nil, err
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(src), &obj); err != nil {
			obj = map[string]any{"raw": src}
		}
		obj["points"] = count
		obj["last_seen_at"] = last
		out = append(out, obj)
	}
	return out, rows.Err()
}

func typedValue(point map[string]any, dataType string) map[string]any {
	camel := kebabToLowerCamel(dataType)
	if m := valueMap(point[camel]); len(m) > 0 {
		return m
	}
	out := map[string]any{}
	keys := make([]string, 0, len(point))
	for k := range point {
		if k != "name" && k != "dataSource" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		out[k] = point[k]
	}
	return out
}

func timesFromValue(v map[string]any) (start, end, civil string) {
	for _, key := range []string{"interval", "sampleTime", "session"} {
		if m := valueMap(v[key]); len(m) > 0 {
			start = firstString(m, "startTime", "physicalTime")
			end = firstString(m, "endTime")
			civil = civilDateFromAny(firstNonNil(m["civilStartTime"], m["civilTime"], m["start"]))
			if civil == "" {
				civil = dateFromTime(start)
			}
			return start, end, civil
		}
	}
	start = firstString(v, "startTime", "physicalTime")
	end = firstString(v, "endTime")
	civil = civilDateFromAny(firstNonNil(v["civilStartTime"], v["civilTime"], v["start"]))
	if civil == "" {
		civil = civilDateFromAny(v["date"])
	}
	return start, end, civil
}

func summarizeValue(dataType, valueJSON string) string {
	var v map[string]any
	if err := json.Unmarshal([]byte(valueJSON), &v); err != nil {
		return ""
	}
	out := map[string]any{}
	switch dataType {
	case "sleep":
		if interval := valueMap(v["interval"]); interval != nil {
			out["start_time"] = firstString(interval, "startTime")
			out["end_time"] = firstString(interval, "endTime")
		}
		if meta := valueMap(v["metadata"]); meta != nil {
			out["nap"] = meta["nap"]
			out["processed"] = meta["processed"]
			out["stages_status"] = meta["stagesStatus"]
		}
		if summary := valueMap(v["summary"]); summary != nil {
			out["minutes_asleep"] = summary["minutesAsleep"]
			out["minutes_awake"] = summary["minutesAwake"]
			out["minutes_in_sleep_period"] = summary["minutesInSleepPeriod"]
			out["stages"] = summary["stagesSummary"]
		}
	case "weight":
		out["weight_grams"] = v["weightGrams"]
	case "body-fat", "oxygen-saturation", "daily-oxygen-saturation":
		out["percentage"] = v["percentage"]
	case "heart-rate":
		out["beats_per_minute"] = v["beatsPerMinute"]
	case "daily-resting-heart-rate":
		out["beats_per_minute"] = v["beatsPerMinute"]
	case "heart-rate-variability", "daily-heart-rate-variability":
		out["rmssd_ms"] = firstNonNil(v["rootMeanSquareOfSuccessiveDifferencesMilliseconds"], v["rmssdMilliseconds"])
	case "daily-respiratory-rate":
		out["breaths_per_minute"] = v["breathsPerMinute"]
	case "respiratory-rate-sleep-summary":
		if full := valueMap(v["fullSleepStats"]); full != nil {
			out["full_sleep_breaths_per_minute"] = full["breathsPerMinute"]
		}
	case "steps":
		out["count"] = firstNonNil(v["count"], v["countSum"])
	case "distance":
		out["meters"] = firstNonNil(v["meters"], v["metersSum"])
		out["millimeters"] = firstNonNil(v["millimeters"], v["millimetersSum"])
	case "active-zone-minutes":
		out["minutes"] = firstNonNil(v["minutes"], v["minutesSum"])
		out["fat_burn_minutes"] = v["sumInFatBurnHeartZone"]
		out["cardio_minutes"] = v["sumInCardioHeartZone"]
		out["peak_minutes"] = v["sumInPeakHeartZone"]
	case "total-calories":
		out["kcal"] = firstNonNil(v["kcal"], v["kcalSum"])
	case "floors":
		out["count"] = firstNonNil(v["count"], v["countSum"])
	default:
		for k, val := range v {
			if k == "interval" || k == "sampleTime" || k == "stages" || k == "createTime" || k == "updateTime" {
				continue
			}
			out[k] = val
		}
	}
	for k, val := range out {
		if val == nil {
			delete(out, k)
		}
	}
	if len(out) == 0 {
		return ""
	}
	b, err := json.Marshal(out)
	if err != nil {
		return ""
	}
	return string(b)
}

func valueMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := stringValue(m[k]); s != "" {
			return s
		}
	}
	return ""
}

func firstNonNil(vals ...any) any {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}

func civilDateFromAny(v any) string {
	m := valueMap(v)
	if len(m) == 0 {
		return ""
	}
	d := valueMap(m["date"])
	if len(d) == 0 {
		d = m
	}
	y, okY := numberAsInt(d["year"])
	mo, okM := numberAsInt(d["month"])
	day, okD := numberAsInt(d["day"])
	if !okY || !okM || !okD {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", y, mo, day)
}

func numberAsInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}

func dateFromTime(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return ""
}

func kebabToLowerCamel(s string) string {
	parts := strings.Split(s, "-")
	for i := 1; i < len(parts); i++ {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, "")
}
