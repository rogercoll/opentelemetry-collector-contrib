package xfilterprocessor

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type Config struct {
	// ErrorMode determines how the processor reacts to errors that occur while processing an OTTL condition.
	// Valid values are `ignore` and `propagate`.
	// `ignore` means the processor ignores errors returned by conditions and continues on to the next condition. This is the recommended mode.
	// `propagate` means the processor returns the error up the pipeline.  This will result in the payload being dropped from the collector.
	// The default value is `propagate`.
	ErrorMode ottl.ErrorMode `mapstructure:"error_mode"`

	// WatchObservers are the extensions to listen to endpoints for dynamic
	// filtering.
	WatchObservers []component.ID `mapstructure:"watch_observers"`

	Profiles ProfileFilters `mapstructure:"profiles"`

	profileFunctions map[string]ottl.Factory[ottlprofile.TransformContext]
}

// ProfileFilters filters by OTTL conditions
type ProfileFilters struct {
	_ struct{} // prevent unkeyed literals

	// ProfileConditions is a list of OTTL conditions for an ottlprofile context.
	// If any condition resolves to true, the profile will be dropped.
	// Supports `and`, `or`, and `()`
	ProfileConditions []string `mapstructure:"profile"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the processor configuration is valid
func (cfg *Config) Validate() error {
	var errors error

	if cfg.Profiles.ProfileConditions != nil {
		_, err := filterottl.NewBoolExprForProfile(cfg.Profiles.ProfileConditions, cfg.profileFunctions, ottl.PropagateError, component.TelemetrySettings{Logger: zap.NewNop()})
		errors = multierr.Append(errors, err)
	}

	return errors
}
