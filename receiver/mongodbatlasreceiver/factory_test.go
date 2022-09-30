// Copyright  OpenTelemetry Authors
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

package mongodbatlasreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	ft := factory.Type()
	require.EqualValues(t, typeStr, ft)
}

func TestBadAlertsReceiver(t *testing.T) {
	conf := createDefaultConfig()
	cfg := conf.(*Config)

	cfg.Alerts.Enabled = true
	cfg.Alerts.TLS = &configtls.TLSServerSetting{
		ClientCAFile: "/not/a/file",
	}
	params := componenttest.NewNopReceiverCreateSettings()

	_, err := createCombinedLogReceiver(context.Background(), params, cfg, consumertest.NewNop())
	require.Error(t, err)
	require.ErrorContains(t, err, "unable to create a MongoDB Atlas Alerts Receiver")
}
