// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package profilingsessionsobserver

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/extension"

	"github.com/elastic/opentelemetry-lib/config/configelasticsearch"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/profilingsessionsobserver/internal/metadata"
)

const (
	defaultTopicsSyncInterval = 5 * time.Second
)

// NewFactory should be called to create a factory with default values.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		component.StabilityLevelBeta,
	)
}

func createDefaultConfig() component.Config {
	defaultElasticSearchClient := configelasticsearch.NewDefaultClientConfig()
	httpCfg := confighttp.NewDefaultServerConfig()
	httpCfg.Endpoint = "localhost:4320"

	return &Config{
		Source: SourceConfig{
			Elasticsearch: &ElasticsearchFetcherConfig{
				ClientConfig: defaultElasticSearchClient,
			},
		},
	}
}

func createExtension(
	_ context.Context,
	settings extension.Settings,
	cfg component.Config,
) (extension.Extension, error) {
	config := cfg.(*Config)
	return newObserver(settings.TelemetrySettings, config)
}
