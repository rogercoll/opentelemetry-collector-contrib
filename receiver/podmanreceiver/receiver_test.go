// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package podmanreceiver

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"
)

func TestNewReceiver(t *testing.T) {
	config := &Config{
		Endpoint: "unix:///run/some.sock",
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 1 * time.Second,
			InitialDelay:       time.Second,
		},
	}
	mr := newMetricsReceiver(receivertest.NewNopCreateSettings(), config, nil)
	assert.NotNil(t, mr)
}

func TestErrorsInStart(t *testing.T) {
	recv := newMetricsReceiver(receivertest.NewNopCreateSettings(), &Config{}, nil)
	assert.NotNil(t, recv)
	err := recv.start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)
	assert.Equal(t, "config.Endpoint must be specified", err.Error())

	recv = newMetricsReceiver(receivertest.NewNopCreateSettings(), &Config{Endpoint: "someEndpoint"}, nil)
	assert.NotNil(t, recv)
	err = recv.start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)
	assert.Equal(t, "config.CollectionInterval must be specified", err.Error())
}

func TestScraperLoop(t *testing.T) {
	cfg := createDefaultConfig()
	cfg.CollectionInterval = 100 * time.Millisecond

	client := make(mockClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := newMetricsReceiver(receivertest.NewNopCreateSettings(), cfg, client.factory)
	assert.NotNil(t, r)

	go func() {
		sampleStats := genContainerStats()
		client <- containerStatsReport{
			Stats: []containerStats{
				*sampleStats,
			},
			Error: containerStatsReportError{},
		}
	}()

	assert.NoError(t, r.start(ctx, componenttest.NewNopHost()))

	md, err := r.scrape(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, md.ResourceMetrics().Len())

	assertStatsEqualToMetrics(t, genContainerStats(), md)

	assert.NoError(t, r.shutdown(ctx))
}

type mockClient chan containerStatsReport

func (c mockClient) factory(_ *zap.Logger, _ *Config) (PodmanClient, error) {
	return c, nil
}

func (c mockClient) stats(context.Context, url.Values) ([]containerStats, error) {
	report := <-c
	if report.Error.Message != "" {
		return nil, errors.New(report.Error.Message)
	}
	return report.Stats, nil
}

func (c mockClient) ping(context.Context) error {
	return nil
}

type mockConsumer chan pmetric.Metrics

func (c mockClient) list(context.Context, url.Values) ([]container, error) {
	return []container{{ID: "c1", Image: "localimage"}}, nil
}

func (c mockClient) events(context.Context, url.Values) (<-chan event, <-chan error) {
	return nil, nil
}

func (m mockConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (m mockConsumer) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	m <- md
	return nil
}
