package groupreceiver

import (
	"github.com/spf13/cast"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
)

type Config struct {
	cfg map[component.ID]map[string]any
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

	for subreceiverKey := range componentParser.ToStringMap() {
		cfgSection := cast.ToStringMap(componentParser.Get(subreceiverKey))
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
