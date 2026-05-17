package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/fdsouvenir/ghcli/internal/output"
	"github.com/fdsouvenir/ghcli/internal/store"
)

func queryCmd(name, short string, dataTypes []string, rollups bool) *cobra.Command {
	var since string
	var until string
	var limit int
	var includeRaw bool
	var typeCSV string
	c := &cobra.Command{
		Use:   name,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalContext(context.Background())
			defer cancel()
			st, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer st.Close()
			types := dataTypes
			if typeCSV != "" {
				types = splitCSV(typeCSV)
			}
			pointTypes := types
			if name == "daily" {
				pointTypes = dailyPointTypes(types)
			}
			var rows []store.DataPointRow
			if name == "daily" && len(pointTypes) == 0 {
				rows = nil
			} else {
				qrows, err := st.QueryDataPoints(ctx, pointTypes, since, until, limit)
				if err != nil {
					return err
				}
				rows = qrows
			}
			if rollups {
				rs, err := st.QueryRollups(ctx, types, since, until, limit)
				if err != nil {
					return err
				}
				rows = append(rows, rs...)
			}
			if !includeRaw {
				for i := range rows {
					rows[i].RawJSON = ""
				}
			}
			if flags.jsonOut {
				return output.JSON(cmd.OutOrStdout(), rows)
			}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				val := firstNonEmpty(r.SummaryJSON, r.ValueJSON)
				if !flags.full {
					val = output.Truncate(compactJSON(val), 80)
				}
				table = append(table, []string{r.DataType, firstNonEmpty(r.CivilDate, r.StartTime), r.RecordKind, val})
			}
			return output.Table(cmd.OutOrStdout(), []string{"type", "time", "kind", "value"}, table)
		},
	}
	c.Flags().StringVar(&since, "since", "", "filter start date/time")
	c.Flags().StringVar(&until, "until", "", "filter end date/time")
	c.Flags().IntVar(&limit, "limit", 100, "maximum rows")
	c.Flags().BoolVar(&includeRaw, "raw", false, "include raw point JSON in --json output")
	c.Flags().StringVar(&typeCSV, "type", "", "override data type list")
	return c
}

func dailyPointTypes(types []string) []string {
	var out []string
	for _, typ := range types {
		if strings.HasPrefix(typ, "daily-") {
			out = append(out, typ)
		}
	}
	return out
}

func accountCmd(name, short, kind string) *cobra.Command {
	var limit int
	c := &cobra.Command{
		Use:   name,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalContext(context.Background())
			defer cancel()
			st, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer st.Close()
			switch kind {
			case "devices":
				srcs, err := st.QuerySources(ctx, limit)
				if err != nil {
					return err
				}
				if flags.jsonOut {
					return output.JSON(cmd.OutOrStdout(), srcs)
				}
				rows := make([][]string, 0, len(srcs))
				for _, src := range srcs {
					rows = append(rows, []string{
						stringish(src["platform"]),
						nestedString(src, "device", "displayName"),
						fmt.Sprint(src["points"]),
						output.FormatTime(int64FromAny(src["last_seen_at"])),
					})
				}
				return output.Table(cmd.OutOrStdout(), []string{"platform", "device", "points", "last_seen"}, rows)
			default:
				a, err := st.Account(ctx)
				if err != nil {
					return err
				}
				var body string
				if kind == "settings" {
					body = a.SettingsJSON
				} else {
					body = a.ProfileJSON
				}
				if flags.jsonOut {
					var obj any
					if err := json.Unmarshal([]byte(body), &obj); err != nil {
						obj = map[string]string{"raw": body}
					}
					return output.JSON(cmd.OutOrStdout(), obj)
				}
				fmt.Fprintln(cmd.OutOrStdout(), compactJSON(body))
				return nil
			}
		},
	}
	c.Flags().IntVar(&limit, "limit", 100, "maximum rows")
	return c
}

func exportCmd() *cobra.Command {
	var since string
	var until string
	var limit int
	var typeCSV string
	c := &cobra.Command{
		Use:   "export",
		Short: "Export local archive records as JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalContext(context.Background())
			defer cancel()
			st, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer st.Close()
			rows, err := st.QueryDataPoints(ctx, splitCSV(typeCSV), since, until, limit)
			if err != nil {
				return err
			}
			rollups, err := st.QueryRollups(ctx, splitCSV(typeCSV), since, until, limit)
			if err != nil {
				return err
			}
			return output.JSON(cmd.OutOrStdout(), map[string]any{
				"data_points": rows,
				"rollups":     rollups,
			})
		},
	}
	c.Flags().StringVar(&since, "since", "", "filter start date/time")
	c.Flags().StringVar(&until, "until", "", "filter end date/time")
	c.Flags().IntVar(&limit, "limit", 10000, "maximum rows per section")
	c.Flags().StringVar(&typeCSV, "type", "", "comma-separated data type allowlist")
	return c
}

func compactJSON(s string) string {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return s
	}
	return string(b)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func stringish(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func nestedString(m map[string]any, path ...string) string {
	var cur any = m
	for _, p := range path {
		asMap, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = asMap[p]
	}
	return stringish(cur)
}

func int64FromAny(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	default:
		return 0
	}
}

func normalizeTypeList(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
