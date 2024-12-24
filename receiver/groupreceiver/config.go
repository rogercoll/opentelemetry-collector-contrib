package groupreceiver

import (
	"fmt"

	"github.com/spf13/cast"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
)

const (
	// receiversConfigKey is the config key name used to specify the subreceivers.
	receiversConfigKey = "configs"
)

type Config struct {
	cfg map[component.ID]map[string]any `mapstructure:"configs"`
}

var _ component.Config = (*Config)(nil)

func (config Config) Validate() error {
	return nil
}

func (cfg *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		// Nothing to do if there is no config given.
		return nil
	}

	if err := componentParser.Unmarshal(cfg, confmap.WithIgnoreUnused()); err != nil {
		return err
	}

	receiversCfg, err := componentParser.Sub(receiversConfigKey)
	if err != nil {
		return fmt.Errorf("unable to extract key %v: %w", receiversConfigKey, err)
	}

	for subreceiverKey := range receiversCfg.ToStringMap() {
		cfgSection := cast.ToStringMap(receiversCfg.Get(subreceiverKey))
		id := component.ID{}
		if err := id.UnmarshalText([]byte(subreceiverKey)); err != nil {
			return err
		}
		// fmt.Println(id.String())
		// fmt.Println(len(cfgSection))

		cfg.cfg[id] = cfgSection
	}

	return nil
}
