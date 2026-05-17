package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/fdsouvenir/ghcli/internal/output"
	ghsync "github.com/fdsouvenir/ghcli/internal/sync"
)

func syncCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "sync",
		Short: "Pull Google Health data into the local SQLite archive",
	}
	c.AddCommand(syncOnceCmd())
	c.AddCommand(syncBackfillCmd())
	c.AddCommand(syncInstallSystemdCmd())
	return c
}

func syncOnceCmd() *cobra.Command {
	var fastOnly bool
	var rollups bool
	var typeCSV string
	var sinceText string
	var archiveRaw bool
	c := &cobra.Command{
		Use:   "once",
		Short: "Sync the current efficient window",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalContext(context.Background())
			defer cancel()
			api, err := authClient(ctx)
			if err != nil {
				return err
			}
			st, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer st.Close()
			if err := ghsync.SyncAccount(ctx, api, st, archiveRaw); err != nil {
				return err
			}
			since := time.Now().Add(-24 * time.Hour)
			if sinceText != "" {
				since, err = parseTimeOrDate(sinceText)
				if err != nil {
					return err
				}
			}
			res, err := ghsync.Run(ctx, api, st, ghsync.Options{
				Since:      since,
				Until:      time.Now(),
				FastOnly:   fastOnly,
				Rollups:    rollups,
				ArchiveRaw: archiveRaw,
				DataTypes:  splitCSV(typeCSV),
			})
			if flags.jsonOut {
				_ = output.JSON(cmd.OutOrStdout(), res)
			} else {
				renderSyncResult(cmd, res)
			}
			return err
		},
	}
	c.Flags().BoolVar(&fastOnly, "fast-only", true, "sync only high-freshness current-day data types")
	c.Flags().BoolVar(&rollups, "rollups", true, "also request daily rollups where supported")
	c.Flags().StringVar(&typeCSV, "type", "", "comma-separated data type allowlist")
	c.Flags().StringVar(&sinceText, "since", "", "override window start (YYYY-MM-DD or RFC3339)")
	c.Flags().BoolVar(&archiveRaw, "archive-raw", false, "also store raw Google Health API response bodies")
	return c
}

func syncBackfillCmd() *cobra.Command {
	var sinceText string
	var untilText string
	var rollups bool
	var typeCSV string
	var archiveRaw bool
	c := &cobra.Command{
		Use:   "backfill",
		Short: "Backfill a historical date/time window",
		RunE: func(cmd *cobra.Command, args []string) error {
			if sinceText == "" || untilText == "" {
				return fmt.Errorf("--since and --until are required")
			}
			ctx, cancel := signalContext(context.Background())
			defer cancel()
			since, err := parseTimeOrDate(sinceText)
			if err != nil {
				return err
			}
			until, err := parseTimeOrDate(untilText)
			if err != nil {
				return err
			}
			api, err := authClient(ctx)
			if err != nil {
				return err
			}
			st, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer st.Close()
			if err := ghsync.SyncAccount(ctx, api, st, archiveRaw); err != nil {
				return err
			}
			res, err := ghsync.Run(ctx, api, st, ghsync.Options{
				Since:      since,
				Until:      until,
				FastOnly:   false,
				Rollups:    rollups,
				ArchiveRaw: archiveRaw,
				DataTypes:  splitCSV(typeCSV),
			})
			if flags.jsonOut {
				_ = output.JSON(cmd.OutOrStdout(), res)
			} else {
				renderSyncResult(cmd, res)
			}
			return err
		},
	}
	c.Flags().StringVar(&sinceText, "since", "", "window start (YYYY-MM-DD or RFC3339)")
	c.Flags().StringVar(&untilText, "until", "", "window end (YYYY-MM-DD or RFC3339)")
	c.Flags().BoolVar(&rollups, "rollups", true, "request daily rollups where supported")
	c.Flags().StringVar(&typeCSV, "type", "", "comma-separated data type allowlist")
	c.Flags().BoolVar(&archiveRaw, "archive-raw", false, "also store raw Google Health API response bodies")
	return c
}

func syncInstallSystemdCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install-systemd",
		Short: "Print user systemd unit templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprint(cmd.OutOrStdout(), `[Unit]
Description=ghcli Google Health sync

[Service]
Type=oneshot
ExecStart=ghcli sync once --fast-only
`)
			fmt.Fprint(cmd.OutOrStdout(), `[Unit]
Description=Run ghcli Google Health sync every 15 minutes

[Timer]
OnBootSec=2m
OnUnitActiveSec=15m
Persistent=true

[Install]
WantedBy=timers.target
`)
			return nil
		},
	}
}

func renderSyncResult(cmd *cobra.Command, res ghsync.Result) {
	fmt.Fprintf(cmd.OutOrStdout(), "window: %s -> %s\n", res.WindowStart, res.WindowEnd)
	for _, tr := range res.Types {
		status := "ok"
		if tr.Error != "" {
			status = tr.Error
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%-36s pages=%d points=%d rollups=%d %s\n", tr.DataType, tr.Pages, tr.Points, tr.Rollups, status)
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseTimeOrDate(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid date/time %q; use YYYY-MM-DD or RFC3339", s)
}
