// Copyright 2020 OpenTelemetry Authors
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

//go:build !windows
// +build !windows

package podmanreceiver

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/podman"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"
)

func TestNewReceiver(t *testing.T) {
	config := &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 1 * time.Second,
		},
		LibPodConfig: podman.LibPodConfig{
			Endpoint: "unix:///run/some.sock",
		},
	}
	nextConsumer := consumertest.NewNop()
	mr, err := newReceiver(context.Background(), receivertest.NewNopCreateSettings(), config, nextConsumer, nil)

	assert.NotNil(t, mr)
	assert.Nil(t, err)
}

func TestNewReceiverErrors(t *testing.T) {
	r, err := newReceiver(context.Background(), receivertest.NewNopCreateSettings(), &Config{}, consumertest.NewNop(), nil)
	assert.Nil(t, r)
	require.Error(t, err)
	assert.Equal(t, "config.Endpoint must be specified", err.Error())

	r, err = newReceiver(context.Background(), receivertest.NewNopCreateSettings(), &Config{LibPodConfig: podman.LibPodConfig{Endpoint: "someEndpoint"}}, consumertest.NewNop(), nil)
	assert.Nil(t, r)
	require.Error(t, err)
	assert.Equal(t, "config.CollectionInterval must be specified", err.Error())
}

func TestScraperLoop(t *testing.T) {
	cfg := createDefaultConfig()
	cfg.CollectionInterval = 100 * time.Millisecond

	client := make(mockClient)
	consumer := make(mockConsumer)

	r, err := newReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, consumer, client.factory)
	require.NoError(t, err)
	assert.NotNil(t, r)

	go func() {
		client <- podman.ContainerStatsReport{
			Stats: []podman.ContainerStats{{
				ContainerID: "c1",
			}},
			Error: podman.ContainerStatsReportError{},
		}
	}()

	assert.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))

	md := <-consumer
	assert.Equal(t, md.ResourceMetrics().Len(), 1)

	assert.NoError(t, r.Shutdown(context.Background()))
}

type mockClient chan podman.ContainerStatsReport

func (c mockClient) factory(logger *zap.Logger, cfg *podman.LibPodConfig) (podman.Client, error) {
	return c, nil
}

func (c mockClient) Stats(context.Context, url.Values) ([]podman.ContainerStats, error) {
	report := <-c
	if report.Error.Message != "" {
		return nil, errors.New(report.Error.Message)
	}
	return report.Stats, nil
}

func (c mockClient) Ping(context.Context) error {
	return nil
}

type mockConsumer chan pmetric.Metrics

func (c mockClient) List(context.Context, url.Values) ([]podman.Container, error) {
	return []podman.Container{{ID: "c1"}}, nil
}

func (c mockClient) Events(context.Context, url.Values) (<-chan podman.Event, <-chan error) {
	return nil, nil
}

func (m mockConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (m mockConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	m <- md
	return nil
}
