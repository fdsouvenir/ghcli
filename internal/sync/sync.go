// Package sync pulls Google Health data into the local archive.
package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/fdsouvenir/fbitcli/internal/health"
	"github.com/fdsouvenir/fbitcli/internal/store"
)

// Options controls a sync pass.
type Options struct {
	Since      time.Time
	Until      time.Time
	FastOnly   bool
	PageSize   int
	Rollups    bool
	ArchiveRaw bool
	DataTypes  []string
}

// Result describes a sync pass.
type Result struct {
	StartedAt   time.Time    `json:"started_at"`
	FinishedAt  time.Time    `json:"finished_at"`
	WindowStart string       `json:"window_start"`
	WindowEnd   string       `json:"window_end"`
	Types       []TypeResult `json:"types"`
	Errors      []string     `json:"errors,omitempty"`
}

type TypeResult struct {
	DataType string `json:"data_type"`
	Pages    int    `json:"pages"`
	Points   int    `json:"points"`
	Rollups  int    `json:"rollups"`
	Error    string `json:"error,omitempty"`
}

// Run performs one pull sync.
func Run(ctx context.Context, api *health.Client, st *store.Store, opt Options) (Result, error) {
	if opt.PageSize <= 0 {
		opt.PageSize = 1000
	}
	if opt.Until.IsZero() {
		opt.Until = time.Now()
	}
	if opt.Since.IsZero() {
		opt.Since = opt.Until.Add(-24 * time.Hour)
	}
	selected := selectDataTypes(opt)
	res := Result{
		StartedAt:   time.Now().UTC(),
		WindowStart: opt.Since.Format(time.RFC3339),
		WindowEnd:   opt.Until.Format(time.RFC3339),
	}
	_ = st.MarkSyncAttempt(ctx)

	for _, dt := range selected {
		tr := syncDataType(ctx, api, st, dt, opt)
		res.Types = append(res.Types, tr)
		if tr.Error != "" {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %s", tr.DataType, tr.Error))
		}
	}
	res.FinishedAt = time.Now().UTC()
	if len(res.Errors) > 0 {
		_ = st.MarkSyncError(ctx, res.Errors[0])
		return res, fmt.Errorf("%d data type(s) failed", len(res.Errors))
	}
	if err := st.MarkSyncSuccess(ctx); err != nil {
		return res, err
	}
	return res, nil
}

// SyncAccount refreshes identity/profile/settings.
func SyncAccount(ctx context.Context, api *health.Client, st *store.Store, archiveRaw bool) error {
	if r, err := api.GetIdentity(ctx); err != nil {
		return err
	} else {
		if archiveRaw {
			if err := st.SaveRawPayload(ctx, rawFromResponse(r, "", "", "")); err != nil {
				return err
			}
		}
		if err := st.SaveIdentity(ctx, r.Body); err != nil {
			return err
		}
	}
	if r, err := api.GetProfile(ctx); err == nil {
		if archiveRaw {
			_ = st.SaveRawPayload(ctx, rawFromResponse(r, "", "", ""))
		}
		_ = st.SaveProfile(ctx, r.Body)
	}
	if r, err := api.GetSettings(ctx); err == nil {
		if archiveRaw {
			_ = st.SaveRawPayload(ctx, rawFromResponse(r, "", "", ""))
		}
		_ = st.SaveSettings(ctx, r.Body)
	}
	return nil
}

func saveRawIfEnabled(ctx context.Context, st *store.Store, opt Options, r health.Response, dataType, start, end string) error {
	if !opt.ArchiveRaw {
		if err := st.UpdateRateLimit(ctx, r.Headers); err != nil {
			return err
		}
		return st.TouchRawActivity(ctx, r.FetchedAt)
	}
	return st.SaveRawPayload(ctx, rawFromResponse(r, dataType, start, end))
}

func syncDataType(ctx context.Context, api *health.Client, st *store.Store, dt health.DataType, opt Options) TypeResult {
	tr := TypeResult{DataType: dt.Name}
	if dt.SupportsList {
		filter := buildFilter(dt, opt.Since, opt.Until)
		page := ""
		for {
			r, err := api.ListDataPoints(ctx, dt.Name, filter, page, opt.PageSize)
			if err != nil {
				tr.Error = err.Error()
				_ = st.MarkDataTypeSync(ctx, dt.Name, opt.Since.Format(time.RFC3339), opt.Until.Format(time.RFC3339), tr.Points, err)
				return tr
			}
			tr.Pages++
			if err := saveRawIfEnabled(ctx, st, opt, r, dt.Name, opt.Since.Format(time.RFC3339), opt.Until.Format(time.RFC3339)); err != nil {
				tr.Error = err.Error()
				return tr
			}
			n, next, err := st.IngestDataPoints(ctx, dt.Name, dt.RecordKind, r.Body)
			if err != nil {
				tr.Error = err.Error()
				return tr
			}
			tr.Points += n
			if next == "" {
				break
			}
			page = next
		}
	}
	if opt.Rollups && dt.SupportsDailyRollup {
		rollups, err := syncDailyRollups(ctx, api, st, dt, opt, opt.Since, opt.Until)
		if err != nil && tr.Points == 0 {
			tr.Error = err.Error()
			return tr
		}
		tr.Rollups = rollups
	}
	_ = st.MarkDataTypeSync(ctx, dt.Name, opt.Since.Format(time.RFC3339), opt.Until.Format(time.RFC3339), tr.Points, nil)
	return tr
}

func syncDailyRollups(ctx context.Context, api *health.Client, st *store.Store, dt health.DataType, opt Options, since, until time.Time) (int, error) {
	total := 0
	day := dateOnly(since)
	last := dateOnly(until)
	for !day.After(last) {
		start, end := health.CivilDayRange(day)
		r, err := api.DailyRollUp(ctx, dt.Name, start, end, 1)
		if err != nil {
			return total, err
		}
		if err := saveRawIfEnabled(ctx, st, opt, r, dt.Name, day.Format("2006-01-02"), day.Format("2006-01-02")); err != nil {
			return total, err
		}
		n, err := st.IngestRollup(ctx, dt.Name, r.Body)
		if err != nil {
			return total, err
		}
		total += n
		day = day.AddDate(0, 0, 1)
	}
	return total, nil
}

func selectDataTypes(opt Options) []health.DataType {
	allowed := map[string]bool{}
	for _, name := range opt.DataTypes {
		allowed[name] = true
	}
	var out []health.DataType
	for _, dt := range health.DataTypes {
		if len(allowed) > 0 && !allowed[dt.Name] {
			continue
		}
		if opt.FastOnly && !dt.FastSync {
			continue
		}
		out = append(out, dt)
	}
	return out
}

func buildFilter(dt health.DataType, since, until time.Time) string {
	field := dt.FilterName
	switch dt.RecordKind {
	case "sample":
		return fmt.Sprintf(`%s.sample_time.physical_time >= "%s" AND %s.sample_time.physical_time < "%s"`, field, since.UTC().Format(time.RFC3339), field, until.UTC().Format(time.RFC3339))
	case "daily":
		return fmt.Sprintf(`%s.date >= "%s"`, field, since.Format("2006-01-02"))
	case "session":
		return ""
	default:
		return fmt.Sprintf(`%s.interval.start_time >= "%s" AND %s.interval.start_time < "%s"`, field, since.UTC().Format(time.RFC3339), field, until.UTC().Format(time.RFC3339))
	}
}

func rawFromResponse(r health.Response, dataType, start, end string) store.RawPayload {
	return store.RawPayload{
		Method:      r.Method,
		Path:        r.Path,
		Query:       r.Query,
		DataType:    dataType,
		WindowStart: start,
		WindowEnd:   end,
		StatusCode:  r.StatusCode,
		Headers:     r.Headers,
		Body:        r.Body,
		FetchedAt:   r.FetchedAt,
	}
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Local().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Local().Location())
}
