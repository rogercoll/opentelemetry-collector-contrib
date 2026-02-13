// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cpuscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/metadata_legacy"
	"go.opentelemetry.io/collector/confmap"
)

// Config relating to CPU Metric Scraper.
type Config struct {
	DualScheme `mapstructure:",squash"`
}

type DualScheme struct {
	legacy metadata_legacy.MetricsBuilderConfig `mapstructure:",squash"`
	v1     metadata.MetricsBuilderConfig        `mapstructure:",squash"`
}

// Unmarshal a config.Parser into the config struct.
func (cfg *DualScheme) Unmarshal(componentParser *confmap.Conf) error {
	if metadata.ReceiverHostmetricsEmitV1SystemConventionsFeatureGate.IsEnabled() &&
		metadata_legacy.ReceiverHostmetricsEmitV0SystemConventionsFeatureGate.IsEnabled() {

		cfg.v1 = metadata.DefaultMetricsBuilderConfig()
		// WithIgnoreUnused to allow other schemes fields
		err := componentParser.Unmarshal(&cfg.v1, confmap.WithIgnoreUnused())
		if err != nil {
			return err
		}

		cfg.legacy = metadata_legacy.DefaultMetricsBuilderConfig()
		// WithIgnoreUnused to allow other schemes fields
		err = componentParser.Unmarshal(&cfg.legacy, confmap.WithIgnoreUnused())
		if err != nil {
			return err
		}
	} else if metadata.ReceiverHostmetricsEmitV1SystemConventionsFeatureGate.IsEnabled() {
		cfg.v1 = metadata.DefaultMetricsBuilderConfig()
		// Only V1 configuration should be allowed, error otherwise
		err := componentParser.Unmarshal(&cfg.v1)
		if err != nil {
			return err
		}
	} else if metadata_legacy.ReceiverHostmetricsEmitV0SystemConventionsFeatureGate.IsEnabled() {
		cfg.legacy = metadata_legacy.DefaultMetricsBuilderConfig()
		// Only Legacy configuration should be allowed, error otherwise
		err := componentParser.Unmarshal(&cfg.legacy)
		if err != nil {
			return err
		}
	}
	return nil
}
