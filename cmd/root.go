package cmd

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/fdsouvenir/fbitcli/internal/health"
	"github.com/fdsouvenir/fbitcli/internal/paths"
	"github.com/fdsouvenir/fbitcli/internal/store"
)

type globalFlags struct {
	storeDir string
	jsonOut  bool
	readOnly bool
	full     bool
	logLevel string
}

var flags globalFlags

// Version is filled by release builds.
var Version = "dev"

// Root constructs the command tree.
func Root() *cobra.Command {
	root := &cobra.Command{
		Use:           "fbitcli",
		Short:         "Local-first Google Health archive for Fitbit data",
		Long:          "fbitcli syncs Google Health API Fitbit data into a local SQLite archive and exposes read-only query commands.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&flags.storeDir, "store", "", "state directory (default: $XDG_STATE_HOME/fbitcli or ~/.local/state/fbitcli)")
	root.PersistentFlags().BoolVar(&flags.jsonOut, "json", false, "emit JSON")
	root.PersistentFlags().BoolVar(&flags.readOnly, "read-only", true, "agent safety flag; query commands are local/read-only")
	root.PersistentFlags().BoolVar(&flags.full, "full", false, "disable table truncation")
	root.PersistentFlags().StringVar(&flags.logLevel, "log-level", "info", "reserved log verbosity flag")

	root.AddCommand(authCmd())
	root.AddCommand(syncCmd())
	root.AddCommand(doctorCmd())
	root.AddCommand(versionCmd())
	root.AddCommand(exportCmd())
	root.AddCommand(maintenanceCmd())
	root.AddCommand(queryCmd("daily", "Daily rollups and daily records", dailyDataTypes(), true))
	root.AddCommand(queryCmd("activity", "Activity and fitness records", []string{"steps", "distance", "floors", "altitude", "active-minutes", "active-zone-minutes", "activity-level", "sedentary-period", "total-calories", "exercise", "swim-lengths-data", "time-in-heart-rate-zone", "calories-in-heart-rate-zone", "vo2-max", "run-vo2-max", "daily-vo2-max"}, false))
	root.AddCommand(queryCmd("sleep", "Sleep records", []string{"sleep"}, false))
	root.AddCommand(queryCmd("heart", "Heart-rate records", []string{"heart-rate", "daily-resting-heart-rate", "daily-heart-rate-zones", "time-in-heart-rate-zone"}, false))
	root.AddCommand(queryCmd("hrv", "Heart-rate variability records", []string{"heart-rate-variability", "daily-heart-rate-variability"}, false))
	root.AddCommand(queryCmd("spo2", "Oxygen saturation records", []string{"oxygen-saturation", "daily-oxygen-saturation"}, false))
	root.AddCommand(queryCmd("breathing", "Respiratory-rate records", []string{"daily-respiratory-rate", "respiratory-rate-sleep-summary"}, false))
	root.AddCommand(queryCmd("temperature", "Sleep temperature derivations", []string{"daily-sleep-temperature-derivations"}, false))
	root.AddCommand(queryCmd("body", "Body measurement records", []string{"weight", "body-fat", "height"}, false))
	root.AddCommand(queryCmd("exercise", "Exercise records", []string{"exercise"}, false))
	root.AddCommand(queryCmd("nutrition", "Nutrition and hydration records", []string{"hydration-log"}, false))
	root.AddCommand(accountCmd("profile", "Cached Google Health profile", "profile"))
	root.AddCommand(accountCmd("settings", "Cached Google Health settings", "settings"))
	root.AddCommand(accountCmd("devices", "Device/source details observed in archived records", "devices"))
	return root
}

func resolveLayout() (paths.Layout, error) {
	layout, err := paths.Resolve(flags.storeDir)
	if err != nil {
		return paths.Layout{}, err
	}
	if err := layout.EnsureDirs(); err != nil {
		return paths.Layout{}, err
	}
	return layout, nil
}

func openStore(ctx context.Context) (*store.Store, error) {
	layout, err := resolveLayout()
	if err != nil {
		return nil, err
	}
	return store.Open(ctx, layout.Database)
}

func authClient(ctx context.Context) (*health.Client, error) {
	layout, err := resolveLayout()
	if err != nil {
		return nil, err
	}
	creds, err := health.LoadCredentials(layout.Credentials)
	if err != nil {
		return nil, err
	}
	src, err := health.TokenSource(ctx, creds.Config(""), health.DefaultVaultTokenStore())
	if err != nil {
		return nil, err
	}
	return health.NewClient(ctx, src), nil
}

func signalContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-ch:
			cancel()
		case <-ctx.Done():
		}
		signal.Stop(ch)
	}()
	return ctx, cancel
}

var errNoLegacyFitbit = errors.New("legacy Fitbit Web API is intentionally unsupported; use Google Health API only")

func dailyDataTypes() []string {
	return []string{
		"active-minutes",
		"active-zone-minutes",
		"altitude",
		"body-fat",
		"calories-in-heart-rate-zone",
		"daily-heart-rate-variability",
		"daily-heart-rate-zones",
		"daily-oxygen-saturation",
		"daily-respiratory-rate",
		"daily-resting-heart-rate",
		"daily-sleep-temperature-derivations",
		"daily-vo2-max",
		"distance",
		"floors",
		"heart-rate",
		"hydration-log",
		"run-vo2-max",
		"sedentary-period",
		"steps",
		"swim-lengths-data",
		"time-in-heart-rate-zone",
		"total-calories",
		"weight",
	}
}
