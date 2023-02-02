package newrelicobserver

import (
	"fmt"
	"time"
)

const (
	defaultRefreshInterval = 30 * time.Second
)

type Config struct {
	// RefreshInterval determines the frequency at which the observer
	// needs to poll for collecting new information about task containers.
	RefreshInterval time.Duration `mapstructure:"refresh_interval" yaml:"refresh_interval"`

	// LicenseKey determines the frequency at which the observer
	// needs to poll for collecting new information about task containers.
	LicenseKey string `mapstructure:"license_key" yaml:"license_key"`
}

func (c Config) Validate() error {
	if c.LicenseKey == "" {
		return fmt.Errorf("no license key provided")
	}
	return nil
}

func defaultConfig() Config {
	return Config{
		RefreshInterval: defaultRefreshInterval,
		LicenseKey:      "",
	}
}
