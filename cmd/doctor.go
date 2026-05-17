package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/fdsouvenir/fbitcli/internal/health"
	"github.com/fdsouvenir/fbitcli/internal/output"
)

type doctorReport struct {
	StoreRoot       string   `json:"store_root"`
	Database        string   `json:"database"`
	CredentialsPath string   `json:"credentials_path"`
	CredentialsOK   bool     `json:"credentials_ok"`
	TokenStore      string   `json:"token_store"`
	TokenPresent    bool     `json:"token_present"`
	TokenExpiry     string   `json:"token_expiry,omitempty"`
	StoreOpens      bool     `json:"store_opens"`
	SchemaVersion   int      `json:"schema_version,omitempty"`
	HealthUserID    string   `json:"health_user_id,omitempty"`
	LegacyUserID    string   `json:"legacy_user_id,omitempty"`
	RawPayloads     int      `json:"raw_payloads,omitempty"`
	DataPoints      int      `json:"data_points,omitempty"`
	RollupPoints    int      `json:"rollup_points,omitempty"`
	LastSuccess     string   `json:"last_success,omitempty"`
	LastRawPayload  string   `json:"last_raw_payload,omitempty"`
	LastDataPoint   string   `json:"last_data_point,omitempty"`
	Issues          []string `json:"issues,omitempty"`
}

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Inspect auth, store, and freshness state",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalContext(context.Background())
			defer cancel()
			r := runDoctor(ctx)
			if flags.jsonOut {
				return output.JSON(cmd.OutOrStdout(), r)
			}
			renderDoctor(cmd, r)
			if len(r.Issues) > 0 {
				return fmt.Errorf("%d issue(s) detected", len(r.Issues))
			}
			return nil
		},
	}
}

func runDoctor(ctx context.Context) doctorReport {
	r := doctorReport{}
	layout, err := resolveLayout()
	if err != nil {
		r.Issues = append(r.Issues, err.Error())
		return r
	}
	r.StoreRoot = layout.Root
	r.Database = layout.Database
	r.CredentialsPath = layout.Credentials
	r.TokenStore = health.DefaultVaultTokenStore().Describe()
	if _, err := health.LoadCredentials(layout.Credentials); err != nil {
		r.Issues = append(r.Issues, err.Error())
	} else {
		r.CredentialsOK = true
	}
	if tok, err := health.DefaultVaultTokenStore().Load(ctx); err != nil {
		r.Issues = append(r.Issues, "token: "+err.Error())
	} else {
		r.TokenPresent = true
		if !tok.Expiry.IsZero() {
			r.TokenExpiry = tok.Expiry.Format(time.RFC3339)
		}
	}
	st, err := openStore(ctx)
	if err != nil {
		r.Issues = append(r.Issues, "store: "+err.Error())
		return r
	}
	defer st.Close()
	r.StoreOpens = true
	if v, err := st.SchemaVersion(ctx); err == nil {
		r.SchemaVersion = v
	}
	if a, err := st.Account(ctx); err == nil {
		r.HealthUserID = a.HealthUserID
		r.LegacyUserID = a.LegacyUserID
	}
	if n, err := st.Count(ctx, "raw_payloads"); err == nil {
		r.RawPayloads = n
	}
	if n, err := st.Count(ctx, "data_points"); err == nil {
		r.DataPoints = n
	}
	if n, err := st.Count(ctx, "rollup_points"); err == nil {
		r.RollupPoints = n
	}
	if m, err := st.SyncMeta(ctx); err == nil {
		r.LastSuccess = formatRFC(m.LastSuccessAt)
		r.LastRawPayload = formatRFC(m.LastRawPayloadAt)
		r.LastDataPoint = formatRFC(m.LastDataPointAt)
		if m.LastError != "" {
			r.Issues = append(r.Issues, "last sync error: "+m.LastError)
		}
	}
	return r
}

func renderDoctor(cmd *cobra.Command, r doctorReport) {
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, "fbitcli doctor")
	fmt.Fprintln(w, "==============")
	fmt.Fprintf(w, "  store root:       %s\n", r.StoreRoot)
	fmt.Fprintf(w, "  database:         %s\n", r.Database)
	fmt.Fprintf(w, "  credentials:      %s ok=%v\n", r.CredentialsPath, r.CredentialsOK)
	fmt.Fprintf(w, "  token store:      %s present=%v\n", r.TokenStore, r.TokenPresent)
	if r.TokenExpiry != "" {
		fmt.Fprintf(w, "  token expiry:     %s\n", r.TokenExpiry)
	}
	fmt.Fprintf(w, "  store opens:      %v\n", r.StoreOpens)
	fmt.Fprintf(w, "  schema version:   %d\n", r.SchemaVersion)
	fmt.Fprintf(w, "  health user id:   %s\n", r.HealthUserID)
	fmt.Fprintf(w, "  legacy user id:   %s\n", r.LegacyUserID)
	fmt.Fprintf(w, "  raw payloads:     %d\n", r.RawPayloads)
	fmt.Fprintf(w, "  data points:      %d\n", r.DataPoints)
	fmt.Fprintf(w, "  rollup points:    %d\n", r.RollupPoints)
	fmt.Fprintf(w, "  last success:     %s\n", r.LastSuccess)
	fmt.Fprintf(w, "  last raw payload: %s\n", r.LastRawPayload)
	fmt.Fprintf(w, "  last data point:  %s\n", r.LastDataPoint)
	if len(r.Issues) > 0 {
		fmt.Fprintln(os.Stderr, "\nIssues:")
		for _, issue := range r.Issues {
			fmt.Fprintf(os.Stderr, "  - %s\n", issue)
		}
	}
}

func formatRFC(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.UnixMilli(ms).Format(time.RFC3339)
}
