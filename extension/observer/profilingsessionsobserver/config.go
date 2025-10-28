package profilingsessionsobserver

import (
	"github.com/elastic/opentelemetry-lib/config/configelasticsearch"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/xconfmap"
)

type Config struct {
	// Source defines the remote configuration source settings.
	Source SourceConfig `mapstructure:"source"`
}

// TODO: inteface?
type SourceConfig struct {
	// Elasticsearch configures a fetcher that retrieves remote configuration
	// data from an Elasticsearch cluster.
	Elasticsearch *ElasticsearchFetcherConfig `mapstructure:"elasticsearch"`

	// OpampFetcher configures a fetcher that retrieves remote configuration
	// data from an Elasticsearch cluster.
	OpAMPFetcher *OpampFetcher `mapstructure:"opamp"`
}

type ElasticsearchFetcherConfig struct {
	// Elasticsearch client configuration.
	configelasticsearch.ClientConfig `mapstructure:",squash"`

	// // CacheDuration specifies how long the fetched remote configuration for an agent
	// // should be cached before fetching it again from Elasticsearch.
	// CacheDuration time.Duration `mapstructure:"cache_duration"`
}

type OpampFetcher struct{}

var (
	_ xconfmap.Validator  = (*Config)(nil)
	_ xconfmap.Validator  = (*ElasticsearchFetcherConfig)(nil)
	_ confmap.Unmarshaler = (*Config)(nil)
	_ component.Config    = (*Config)(nil)
)

// Validate checks the receiver configuration is valid
func (cfg *ElasticsearchFetcherConfig) Validate() error {
	return nil
}

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	return nil
}

// Unmarshal a confmap.Conf into the config struct.
func (cfg *Config) Unmarshal(conf *confmap.Conf) error {
	// first load the config normally
	err := conf.Unmarshal(cfg)
	if err != nil {
		return err
	}

	return nil
}
