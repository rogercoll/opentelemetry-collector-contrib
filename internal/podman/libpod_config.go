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

package podman // import "github.com/open-telemetry/opentelemetry-collector-contrib//podman"

import (
	"errors"
	"time"
)

type LibPodConfig struct {
	// The URL of the podman server.  Default is "unix:///run/podman/podman.sock"
	Endpoint string `mapstructure:"endpoint"`

	// The maximum amount of time to wait for Podman API responses.  Default is 5s
	Timeout time.Duration `mapstructure:"timeout"`

	APIVersion    string `mapstructure:"api_version"`
	SSHKey        string `mapstructure:"ssh_key"`
	SSHPassphrase string `mapstructure:"ssh_passphrase"`
}

func (config LibPodConfig) Validate() error {
	if config.Endpoint == "" {
		return errors.New("config.Endpoint must be specified")
	}
	return nil
}
