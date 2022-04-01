// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package podmanreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

var _ config.Receiver = (*Config)(nil)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`

	// The URL of the podman server.  Default is "unix:///run/podman/podman.sock"
	Endpoint      string `mapstructure:"endpoint"`
	APIVersion    string `mapstructure:"api_version"`
	SSHKey        string `mapstructure:"ssh_key"`
	SSHPassphrase string `mapstructure:"ssh_passphrase"`
	// A list of filters whose matching images are to be excluded.  Supports literals, globs, and regex.
	ExcludedImages []string `mapstructure:"excluded_images"`
}

func (config Config) Validate() error {
	if config.Endpoint == "" {
		return errors.New("config.Endpoint must be specified")
	}
	if config.CollectionInterval == 0 {
		return errors.New("config.CollectionInterval must be specified")
	}
	return nil
}
