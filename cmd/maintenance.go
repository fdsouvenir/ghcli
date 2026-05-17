package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/fdsouvenir/fbitcli/internal/output"
)

func maintenanceCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "maintenance",
		Short: "Maintain the local archive",
	}
	c.AddCommand(pruneRawCmd())
	return c
}

func pruneRawCmd() *cobra.Command {
	var olderThanText string
	var vacuum bool
	c := &cobra.Command{
		Use:   "prune-raw",
		Short: "Delete raw API response bodies while keeping queryable records",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalContext(context.Background())
			defer cancel()
			st, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer st.Close()
			var cutoff time.Time
			if olderThanText != "" {
				cutoff, err = parseTimeOrDate(olderThanText)
				if err != nil {
					return err
				}
			}
			deleted, err := st.PruneRawPayloads(ctx, cutoff)
			if err != nil {
				return err
			}
			if vacuum {
				if err := st.Vacuum(ctx); err != nil {
					return err
				}
			}
			report := map[string]any{
				"raw_payloads_deleted": deleted,
				"vacuum":               vacuum,
			}
			if !cutoff.IsZero() {
				report["older_than"] = cutoff.Format(time.RFC3339)
			}
			if flags.jsonOut {
				return output.JSON(cmd.OutOrStdout(), report)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted raw payloads: %d\n", deleted)
			if vacuum {
				fmt.Fprintln(cmd.OutOrStdout(), "vacuum: complete")
			}
			return nil
		},
	}
	c.Flags().StringVar(&olderThanText, "older-than", "", "delete only raw payloads fetched before this date/time")
	c.Flags().BoolVar(&vacuum, "vacuum", false, "compact SQLite after deleting raw payloads")
	return c
}
