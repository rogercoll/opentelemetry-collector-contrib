// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofmetricsconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/pprofmetricsconnector"

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/pprofmetricsconnector/internal/metadata"
)

// Config defines configuration for the pprof metrics connector.
type Config struct {
	metadata.MetricsBuilderConfig `mapstructure:",squash"`
	// FlushInterval, when greater than zero, buffers metrics and merges streams that share the same
	// identity (resource, scope, metric, datapoint attributes) before forwarding, on a fixed ticker.
	// This matches Elasticsearch profilingmetricsconnector and reduces duplicate TSDB documents when
	// the same series is exported repeatedly within the window. Set to 0 to forward each export immediately.
	FlushInterval time.Duration `mapstructure:"flush_interval"`
}

var _ xconfmap.Validator = (*Config)(nil)

// Validate checks the connector configuration.
func (c *Config) Validate() error {
	if c.FlushInterval < 0 {
		return fmt.Errorf("flush_interval must be >= 0")
	}
	return nil
}
