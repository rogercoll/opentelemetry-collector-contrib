// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mongodbatlasreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		name        string
		input       Config
		expectedErr string
	}{
		{
			name:  "Empty config",
			input: Config{},
		},
		{
			name: "Valid alerts config",
			input: Config{
				Alerts: AlertConfig{
					Enabled:  true,
					Endpoint: "0.0.0.0:7706",
					Secret:   "some_secret",
					Mode:     alertModeListen,
				},
			},
		},
		{
			name: "Alerts missing endpoint",
			input: Config{
				Alerts: AlertConfig{
					Enabled: true,
					Secret:  "some_secret",
					Mode:    alertModeListen,
				},
			},
			expectedErr: errNoEndpoint.Error(),
		},
		{
			name: "Alerts missing secret",
			input: Config{
				Alerts: AlertConfig{
					Enabled:  true,
					Endpoint: "0.0.0.0:7706",
					Mode:     alertModeListen,
				},
			},
			expectedErr: errNoSecret.Error(),
		},
		{
			name: "Invalid endpoint",
			input: Config{
				Alerts: AlertConfig{
					Enabled:  true,
					Endpoint: "7706",
					Secret:   "some_secret",
					Mode:     alertModeListen,
				},
			},
			expectedErr: "failed to split endpoint into 'host:port' pair",
		},
		{
			name: "TLS config missing key",
			input: Config{
				Alerts: AlertConfig{
					Enabled:  true,
					Endpoint: "0.0.0.0:7706",
					Secret:   "some_secret",
					Mode:     alertModeListen,
					TLS: &configtls.TLSServerSetting{
						TLSSetting: configtls.TLSSetting{
							CertFile: "some_cert_file",
						},
					},
				},
			},
			expectedErr: errNoKey.Error(),
		},
		{
			name: "TLS config missing cert",
			input: Config{
				Alerts: AlertConfig{
					Enabled:  true,
					Endpoint: "0.0.0.0:7706",
					Secret:   "some_secret",
					Mode:     alertModeListen,
					TLS: &configtls.TLSServerSetting{
						TLSSetting: configtls.TLSSetting{
							KeyFile: "some_key_file",
						},
					},
				},
			},
			expectedErr: errNoCert.Error(),
		},
		{
			name: "Valid Logs Config",
			input: Config{
				Logs: LogConfig{
					Enabled: true,
					Projects: []*ProjectConfig{
						{
							Name:            "Project1",
							EnableAuditLogs: false,
						},
					},
				},
			},
		},
		{
			name: "Invalid Logs Config",
			input: Config{
				Logs: LogConfig{
					Enabled: true,
				},
			},
			expectedErr: errNoProjects.Error(),
		},
		{
			name: "Invalid ProjectConfig",
			input: Config{
				Logs: LogConfig{
					Enabled: true,
					Projects: []*ProjectConfig{
						{
							Name:            "Project1",
							EnableAuditLogs: false,
							ExcludeClusters: []string{"cluster1"},
							IncludeClusters: []string{"cluster2"},
						},
					},
				},
			},
			expectedErr: errClusterConfig.Error(),
		},
		{
			name: "Invalid Alerts Retrieval ProjectConfig",
			input: Config{
				Alerts: AlertConfig{
					Enabled: true,
					Mode:    alertModePoll,
					Projects: []*ProjectConfig{
						{
							Name:            "Project1",
							EnableAuditLogs: false,
							ExcludeClusters: []string{"cluster1"},
							IncludeClusters: []string{"cluster2"},
						},
					},
				},
			},
			expectedErr: errClusterConfig.Error(),
		},
		{
			name: "Invalid Alerts Poll No Projects",
			input: Config{
				Alerts: AlertConfig{
					Enabled:  true,
					Mode:     alertModePoll,
					Projects: []*ProjectConfig{},
				},
			},
			expectedErr: errNoProjects.Error(),
		},
		{
			name: "Valid Alerts Config",
			input: Config{
				Alerts: AlertConfig{
					Enabled: true,
					Mode:    alertModePoll,
					Projects: []*ProjectConfig{
						{
							Name: "Project1",
						},
					},
				},
			},
		},
		{
			name: "Invalid Alerts Mode",
			input: Config{
				Alerts: AlertConfig{
					Enabled:  true,
					Mode:     "invalid type",
					Projects: []*ProjectConfig{},
				},
			},
			expectedErr: errNoModeRecognized.Error(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
			if tc.expectedErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
