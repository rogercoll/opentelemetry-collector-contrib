package newrelicobserver

import (
	"context"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"time"
)

const (
	// The value of extension "type" in configuration.
	typeStr component.Type = "newrelic_observer"
)

// NewFactory should be called to create a factory with default values.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		typeStr,
		createDefaultConfig,
		createExtension,
		component.StabilityLevelBeta,
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		RefreshInterval: 5 * time.Second,
	}
}

func createExtension(
	_ context.Context,
	settings extension.CreateSettings,
	cfg component.Config,
) (extension.Extension, error) {
	config := cfg.(*Config)
	return newObserver(settings.Logger, config)
}
