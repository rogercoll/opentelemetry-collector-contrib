// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregatemetrics

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// Consumer merges metrics by stream identity and flushes to the next consumer on a ticker.
// This matches Elasticsearch profilingmetricsconnector: buffering + summation of duplicate
// streams within flush_interval reduces TSDB duplicate-document pressure downstream.
type Consumer struct {
	nextConsumer  consumer.Metrics
	flushInterval time.Duration
	logger        *zap.Logger

	mu           sync.Mutex
	md           pmetric.Metrics
	rmLookup     map[Resource]pmetric.ResourceMetrics
	smLookup     map[Scope]pmetric.ScopeMetrics
	mLookup      map[Metric]pmetric.Metric
	numberLookup map[Stream]pmetric.NumberDataPoint

	cancel context.CancelFunc
}

// New constructs a Consumer. flushInterval must be > 0.
func New(next consumer.Metrics, flushInterval time.Duration, logger *zap.Logger) *Consumer {
	return &Consumer{
		nextConsumer:  next,
		flushInterval: flushInterval,
		logger:        logger,
		md:            pmetric.NewMetrics(),
		rmLookup:      make(map[Resource]pmetric.ResourceMetrics),
		smLookup:      make(map[Scope]pmetric.ScopeMetrics),
		mLookup:       make(map[Metric]pmetric.Metric),
		numberLookup:  make(map[Stream]pmetric.NumberDataPoint),
	}
}

// Start runs the periodic flush loop until ctx is cancelled.
func (c *Consumer) Start(ctx context.Context) error {
	ctx, c.cancel = context.WithCancel(ctx)
	ticker := time.NewTicker(c.flushInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := c.flush(ctx); err != nil && c.logger != nil {
					c.logger.Error("metrics flush failed", zap.Error(err))
				}
			}
		}
	}()
	return nil
}

// Shutdown stops the ticker and flushes remaining metrics.
func (c *Consumer) Shutdown(ctx context.Context) error {
	if c.cancel != nil {
		c.cancel()
	}
	return c.flush(ctx)
}

func (c *Consumer) flush(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.md.ResourceMetrics().Len() == 0 {
		return nil
	}
	err := c.nextConsumer.ConsumeMetrics(ctx, c.md)
	c.md = pmetric.NewMetrics()
	clear(c.rmLookup)
	clear(c.smLookup)
	clear(c.mLookup)
	clear(c.numberLookup)
	return err
}

// Capabilities implements consumer.Metrics.
func (c *Consumer) Capabilities() consumer.Capabilities {
	return c.nextConsumer.Capabilities()
}

// ConsumeMetrics merges incoming metrics into the buffer (same logic as Elasticsearch profilingmetricsconnector).
func (c *Consumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	var errs error
	c.mu.Lock()
	defer c.mu.Unlock()

	md.ResourceMetrics().RemoveIf(func(rm pmetric.ResourceMetrics) bool {
		rm.ScopeMetrics().RemoveIf(func(sm pmetric.ScopeMetrics) bool {
			sm.Metrics().RemoveIf(func(m pmetric.Metric) bool {
				switch m.Type() {
				case pmetric.MetricTypeGauge:
					mClone, metricID := c.getOrCloneMetric(rm, sm, m)
					aggregateNumberDataPoints(m.Gauge().DataPoints(), mClone.Gauge().DataPoints(), metricID, c.numberLookup)
					return true
				case pmetric.MetricTypeSum:
					sum := m.Sum()
					if !sum.IsMonotonic() {
						return false
					}
					if sum.AggregationTemporality() == pmetric.AggregationTemporalityUnspecified {
						return false
					}
					mClone, metricID := c.getOrCloneMetric(rm, sm, m)
					aggregateNumberDataPoints(sum.DataPoints(), mClone.Sum().DataPoints(), metricID, c.numberLookup)
					return true
				default:
					errs = errors.Join(errs, fmt.Errorf("aggregatemetrics: unsupported metric type %v", m.Type()))
					return false
				}
			})
			return sm.Metrics().Len() == 0
		})
		return rm.ScopeMetrics().Len() == 0
	})
	return errs
}

func (c *Consumer) getOrCloneMetric(rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, m pmetric.Metric) (pmetric.Metric, Metric) {
	resID := OfResource(rm.Resource())
	rmClone, ok := c.rmLookup[resID]
	if !ok {
		rmClone = c.md.ResourceMetrics().AppendEmpty()
		rm.Resource().CopyTo(rmClone.Resource())
		rmClone.SetSchemaUrl(rm.SchemaUrl())
		c.rmLookup[resID] = rmClone
	}

	scopeID := OfScope(resID, sm.Scope())
	smClone, ok := c.smLookup[scopeID]
	if !ok {
		smClone = rmClone.ScopeMetrics().AppendEmpty()
		sm.Scope().CopyTo(smClone.Scope())
		smClone.SetSchemaUrl(sm.SchemaUrl())
		c.smLookup[scopeID] = smClone
	}

	metricID := OfMetric(scopeID, m)
	mClone, ok := c.mLookup[metricID]
	if !ok {
		mClone = smClone.Metrics().AppendEmpty()
		mClone.SetName(m.Name())
		mClone.SetDescription(m.Description())
		mClone.SetUnit(m.Unit())

		switch m.Type() {
		case pmetric.MetricTypeGauge:
			mClone.SetEmptyGauge()
		case pmetric.MetricTypeSum:
			src := m.Sum()
			dest := mClone.SetEmptySum()
			dest.SetAggregationTemporality(src.AggregationTemporality())
			dest.SetIsMonotonic(src.IsMonotonic())
		default:
		}

		c.mLookup[metricID] = mClone
	}

	return mClone, metricID
}

func aggregateNumberDataPoints(
	src, dst pmetric.NumberDataPointSlice,
	metricID Metric,
	dpLookup map[Stream]pmetric.NumberDataPoint,
) {
	for i := 0; i < src.Len(); i++ {
		dp := src.At(i)

		streamID := OfStream(metricID, dp)
		existingDP, ok := dpLookup[streamID]
		if !ok {
			dpClone := dst.AppendEmpty()
			dp.CopyTo(dpClone)
			dpLookup[streamID] = dpClone
			continue
		}

		switch existingDP.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			dp.SetIntValue(dp.IntValue() + existingDP.IntValue())
		case pmetric.NumberDataPointValueTypeDouble:
			dp.SetDoubleValue(dp.DoubleValue() + existingDP.DoubleValue())
		}

		ts := existingDP.Timestamp()
		if existingDP.Timestamp() > dp.Timestamp() {
			dp.SetTimestamp(ts)
		}
		dp.CopyTo(existingDP)
	}
}
