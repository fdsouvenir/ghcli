// Package health wraps the Google Health API.
package health

const (
	APIBase = "https://health.googleapis.com/v4"

	ScopeBase = "https://www.googleapis.com/auth/googlehealth"
)

// ReadOnlyScopes is the full read-only Google Health scope set ghcli asks
// for. These are intentionally Google Health scopes only; legacy Fitbit Web
// API scopes are not used anywhere in this repository.
var ReadOnlyScopes = []string{
	ScopeBase + ".activity_and_fitness.readonly",
	ScopeBase + ".health_metrics_and_measurements.readonly",
	ScopeBase + ".location.readonly",
	ScopeBase + ".nutrition.readonly",
	ScopeBase + ".profile.readonly",
	ScopeBase + ".settings.readonly",
	ScopeBase + ".sleep.readonly",
}

// DataType describes a Google Health data type ghcli syncs.
type DataType struct {
	Name                string
	FilterName          string
	RecordKind          string
	ScopeGroup          string
	SupportsList        bool
	SupportsDailyRollup bool
	FastSync            bool
}

// DataTypes mirrors the Google Health data type table as of 2026-05-17.
var DataTypes = []DataType{
	{Name: "active-minutes", FilterName: "active_minutes", RecordKind: "interval", ScopeGroup: "activity_and_fitness", SupportsDailyRollup: true},
	{Name: "active-zone-minutes", FilterName: "active_zone_minutes", RecordKind: "interval", ScopeGroup: "activity_and_fitness", SupportsList: true, SupportsDailyRollup: true, FastSync: true},
	{Name: "activity-level", FilterName: "activity_level", RecordKind: "interval", ScopeGroup: "activity_and_fitness", SupportsList: true, FastSync: true},
	{Name: "altitude", FilterName: "altitude", RecordKind: "interval", ScopeGroup: "activity_and_fitness", SupportsList: true, SupportsDailyRollup: true},
	{Name: "body-fat", FilterName: "body_fat", RecordKind: "sample", ScopeGroup: "health_metrics_and_measurements", SupportsList: true, SupportsDailyRollup: true},
	{Name: "calories-in-heart-rate-zone", FilterName: "calories_in_heart_rate_zone", RecordKind: "interval", ScopeGroup: "activity_and_fitness", SupportsDailyRollup: true},
	{Name: "daily-heart-rate-variability", FilterName: "daily_heart_rate_variability", RecordKind: "daily", ScopeGroup: "health_metrics_and_measurements", SupportsList: true},
	{Name: "daily-heart-rate-zones", FilterName: "daily_heart_rate_zones", RecordKind: "daily", ScopeGroup: "health_metrics_and_measurements"},
	{Name: "daily-oxygen-saturation", FilterName: "daily_oxygen_saturation", RecordKind: "daily", ScopeGroup: "health_metrics_and_measurements", SupportsList: true},
	{Name: "daily-respiratory-rate", FilterName: "daily_respiratory_rate", RecordKind: "daily", ScopeGroup: "health_metrics_and_measurements", SupportsList: true},
	{Name: "daily-resting-heart-rate", FilterName: "daily_resting_heart_rate", RecordKind: "daily", ScopeGroup: "health_metrics_and_measurements", SupportsList: true},
	{Name: "daily-sleep-temperature-derivations", FilterName: "daily_sleep_temperature_derivations", RecordKind: "daily", ScopeGroup: "health_metrics_and_measurements", SupportsList: true},
	{Name: "daily-vo2-max", FilterName: "daily_vo2_max", RecordKind: "daily", ScopeGroup: "activity_and_fitness", SupportsList: true},
	{Name: "distance", FilterName: "distance", RecordKind: "interval", ScopeGroup: "activity_and_fitness", SupportsList: true, SupportsDailyRollup: true, FastSync: true},
	{Name: "exercise", FilterName: "exercise", RecordKind: "session", ScopeGroup: "activity_and_fitness", SupportsList: true},
	{Name: "floors", FilterName: "floors", RecordKind: "interval", ScopeGroup: "activity_and_fitness", SupportsDailyRollup: true},
	{Name: "heart-rate", FilterName: "heart_rate", RecordKind: "sample", ScopeGroup: "health_metrics_and_measurements", SupportsList: true, SupportsDailyRollup: true, FastSync: true},
	{Name: "heart-rate-variability", FilterName: "heart_rate_variability", RecordKind: "sample", ScopeGroup: "health_metrics_and_measurements", SupportsList: true},
	{Name: "height", FilterName: "height", RecordKind: "sample", ScopeGroup: "health_metrics_and_measurements", SupportsList: true},
	{Name: "hydration-log", FilterName: "hydration_log", RecordKind: "session", ScopeGroup: "nutrition", SupportsList: true, SupportsDailyRollup: true},
	{Name: "oxygen-saturation", FilterName: "oxygen_saturation", RecordKind: "sample", ScopeGroup: "health_metrics_and_measurements", SupportsList: true},
	{Name: "respiratory-rate-sleep-summary", FilterName: "respiratory_rate_sleep_summary", RecordKind: "sample", ScopeGroup: "health_metrics_and_measurements", SupportsList: true},
	{Name: "run-vo2-max", FilterName: "run_vo2_max", RecordKind: "sample", ScopeGroup: "activity_and_fitness", SupportsList: true, SupportsDailyRollup: true},
	{Name: "sedentary-period", FilterName: "sedentary_period", RecordKind: "interval", ScopeGroup: "activity_and_fitness", SupportsList: true, SupportsDailyRollup: true},
	{Name: "sleep", FilterName: "sleep", RecordKind: "session", ScopeGroup: "sleep", SupportsList: true},
	{Name: "steps", FilterName: "steps", RecordKind: "interval", ScopeGroup: "activity_and_fitness", SupportsList: true, SupportsDailyRollup: true, FastSync: true},
	{Name: "swim-lengths-data", FilterName: "swim_lengths_data", RecordKind: "interval", ScopeGroup: "activity_and_fitness", SupportsList: true, SupportsDailyRollup: true},
	{Name: "time-in-heart-rate-zone", FilterName: "time_in_heart_rate_zone", RecordKind: "interval", ScopeGroup: "activity_and_fitness", SupportsDailyRollup: true},
	{Name: "total-calories", FilterName: "total_calories", RecordKind: "interval", ScopeGroup: "activity_and_fitness", SupportsDailyRollup: true},
	{Name: "vo2-max", FilterName: "vo2_max", RecordKind: "sample", ScopeGroup: "activity_and_fitness", SupportsList: true},
	{Name: "weight", FilterName: "weight", RecordKind: "sample", ScopeGroup: "health_metrics_and_measurements", SupportsList: true, SupportsDailyRollup: true},
}

// DataTypeByName returns the known data type metadata, if present.
func DataTypeByName(name string) (DataType, bool) {
	for _, dt := range DataTypes {
		if dt.Name == name {
			return dt, true
		}
	}
	return DataType{}, false
}
