// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofmetricsconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/pprofmetricsconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/xconnector"
	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/pprofmetricsconnector/internal/aggregatemetrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/pprofmetricsconnector/internal/metadata"
)

// NewFactory creates a factory for the pprof metrics connector.
func NewFactory() connector.Factory {
	return xconnector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xconnector.WithProfilesToMetrics(createProfilesToMetrics, metadata.ProfilesToMetricsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		FlushInterval:        0,
	}
}

func createProfilesToMetrics(
	_ context.Context,
	set connector.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (xconnector.Profiles, error) {
	c := cfg.(*Config)
	next := nextConsumer
	var agg *aggregatemetrics.Consumer
	if c.FlushInterval > 0 {
		agg = aggregatemetrics.New(nextConsumer, c.FlushInterval, set.Logger)
		next = agg
	}
	return &pprofToMetricsConnector{
		nextConsumer: next,
		agg:          agg,
		config:       c,
		mb:           metadata.NewMetricsBuilder(c.MetricsBuilderConfig, set),
	}, nil
}
