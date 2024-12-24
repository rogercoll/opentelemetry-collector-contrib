package valkeyreceiver

import (
	"context"
	"path/filepath"
	"testing"

	_ "embed"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

//go:embed testdata/all-metrics/raw_info.txt
var raw_info string

type mockClient struct{}

func (mockClient) retrieveInfo(context.Context) (map[string]string, error) {
	return parseRawDataMap(raw_info), nil
}

func (mockClient) close() error {
	return nil
}

var _ client = (*mockClient)(nil)

func TestScrape(t *testing.T) {
	// TODO: change to test table for testing multiple clients output
	goldenDir := filepath.Join("testdata/all-metrics")
	settings := receivertest.NewNopSettings()
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:6379"
	scraper, err := newValkeyScraper(cfg, settings)
	defer scraper.shutdown(context.Background())

	require.NoError(t, err)
	scraper.client = mockClient{}

	metrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expectedMetrics, err := golden.ReadMetrics(filepath.Join(goldenDir, "output-metrics.yaml"))
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, metrics, pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreTimestamp(), pmetrictest.IgnoreStartTimestamp()))
}
