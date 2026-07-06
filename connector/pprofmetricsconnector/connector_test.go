// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofmetricsconnector

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/connector/xconnector"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/testdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/pprofmetricsconnector/internal/metadata"
)

func TestConsumeProfiles_GenerateProfiles(t *testing.T) {
	factory := NewFactory().(xconnector.Factory)
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.FlushInterval = 0
	sink := &consumertest.MetricsSink{}
	conn, err := factory.CreateProfilesToMetrics(t.Context(), connectortest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err)
	c := conn.(*pprofToMetricsConnector)
	require.NoError(t, c.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, c.Shutdown(t.Context())) })

	profiles := testdata.GenerateProfiles(2)
	require.NoError(t, c.ConsumeProfiles(t.Context(), profiles))

	metrics := sink.AllMetrics()
	require.Len(t, metrics, 1)
	rm := metrics[0].ResourceMetrics().At(0)
	require.Equal(t, 1, rm.ScopeMetrics().Len())
	sm := rm.ScopeMetrics().At(0)
	require.Equal(t, metadata.ScopeName, sm.Scope().Name())
	require.NotZero(t, metricDataPointCount(sm.Metrics()))
}

func metricDataPointCount(ms pmetric.MetricSlice) int {
	n := 0
	for i := 0; i < ms.Len(); i++ {
		m := ms.At(i)
		switch m.Type() {
		case pmetric.MetricTypeGauge:
			n += m.Gauge().DataPoints().Len()
		case pmetric.MetricTypeSum:
			n += m.Sum().DataPoints().Len()
		case pmetric.MetricTypeHistogram:
			n += m.Histogram().DataPoints().Len()
		case pmetric.MetricTypeExponentialHistogram:
			n += m.ExponentialHistogram().DataPoints().Len()
		case pmetric.MetricTypeSummary:
			n += m.Summary().DataPoints().Len()
		case pmetric.MetricTypeEmpty:
		}
	}
	return n
}
