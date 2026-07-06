// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofmetricsconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/pprofmetricsconnector"

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/pprofmetricsconnector/internal/aggregatemetrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/pprofmetricsconnector/internal/metadata"
)

// profileFrameTypeKey is the attribute key for stack frame runtime/language kind (OTel semconv).
const profileFrameTypeKey = "profile.frame.type"

type pprofToMetricsConnector struct {
	nextConsumer consumer.Metrics
	agg          *aggregatemetrics.Consumer
	config       *Config
	mb           *metadata.MetricsBuilder

	// mb and pdata Metric MoveTo are not safe for concurrent use; scrapers may call ConsumeProfiles in parallel.
	mu sync.Mutex
}

func (c *pprofToMetricsConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *pprofToMetricsConnector) ConsumeProfiles(ctx context.Context, profiles pprofile.Profiles) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	dict := profiles.Dictionary()
	for _, rp := range profiles.ResourceProfiles().All() {
		profileSeq := 0
		for scopeIdx, sp := range rp.ScopeProfiles().All() {
			for profIdx, prof := range sp.Profiles().All() {
				c.recordProfileMetrics(
					dict,
					prof,
					stableProfileKey(prof, scopeIdx, profIdx),
					metricDataPointTimestamp(prof, profileSeq),
				)
				profileSeq++
			}
		}
		if err := c.nextConsumer.ConsumeMetrics(ctx, c.mb.Emit(metadata.WithResource(rp.Resource()))); err != nil {
			return err
		}
	}
	return nil
}

func (c *pprofToMetricsConnector) Start(ctx context.Context, _ component.Host) error {
	if c.agg != nil {
		return c.agg.Start(ctx)
	}
	return nil
}

func (c *pprofToMetricsConnector) Shutdown(ctx context.Context) error {
	if c.agg != nil {
		return c.agg.Shutdown(ctx)
	}
	return nil
}

func stableProfileKey(prof pprofile.Profile, scopeIdx, profIdx int) string {
	id := prof.ProfileID()
	if !id.IsEmpty() {
		return id.String()
	}
	return fmt.Sprintf("scope_%d_profile_%d", scopeIdx, profIdx)
}

// metricDataPointTimestamp sets each profile's metric datapoint time. Profile.Time() is the
// observation time on the wire; scrapers often assign the same nanosecond to every profile in a
// batch. Using that value verbatim for all metrics can yield identical (dimensions, @timestamp)
// keys for TSDB stores. We add profileSeq nanoseconds so each profile stays in the same window
// but timestamps remain distinct; profileSeq resets per ResourceMetrics.
func metricDataPointTimestamp(prof pprofile.Profile, profileSeq int) pcommon.Timestamp {
	base := uint64(prof.Time())
	if profileSeq <= 0 {
		return prof.Time()
	}
	return pcommon.Timestamp(base + uint64(profileSeq))
}

func (c *pprofToMetricsConnector) recordProfileMetrics(
	dict pprofile.ProfilesDictionary,
	prof pprofile.Profile,
	profileKey string,
	ts pcommon.Timestamp,
) {
	st := prof.SampleType()
	strTable := dict.StringTable()
	periodType := prof.PeriodType()

	sampleType := strAt(strTable, int(st.TypeStrindex()))
	sampleUnit := strAt(strTable, int(st.UnitStrindex()))
	periodTypeStr := strAt(strTable, int(periodType.TypeStrindex()))
	periodUnitStr := strAt(strTable, int(periodType.UnitStrindex()))
	sampleOrigin := sampleType + "_" + sampleUnit

	var valueSum int64
	nSamples := 0
	for _, sample := range prof.Samples().All() {
		nSamples++
		for _, v := range sample.Values().All() {
			valueSum += v
		}
	}

	c.mb.RecordPprofProfileSampleValueSumDataPoint(ts, valueSum, periodTypeStr, periodUnitStr, profileKey, sampleType, sampleUnit)
	c.mb.RecordPprofProfileSampleCountDataPoint(ts, int64(nSamples), periodTypeStr, periodUnitStr, profileKey, sampleType, sampleUnit)
	c.mb.RecordPprofProfilePeriodDataPoint(ts, prof.Period(), periodTypeStr, periodUnitStr, profileKey, sampleType, sampleUnit)
	c.mb.RecordPprofProfileDurationNanosecondsDataPoint(ts, safeUint64ToInt64(prof.DurationNano()), periodTypeStr, periodUnitStr, profileKey, sampleType, sampleUnit)

	frameCounts := countFrameTypes(dict, prof)
	for ft, cnt := range frameCounts {
		c.mb.RecordPprofProfileFrameTypeSamplesDataPoint(ts, cnt, ft, periodTypeStr, periodUnitStr, profileKey, sampleOrigin, sampleType, sampleUnit)
	}
}

func countFrameTypes(dict pprofile.ProfilesDictionary, prof pprofile.Profile) map[string]int64 {
	out := make(map[string]int64)
	stackTable := dict.StackTable()
	locTable := dict.LocationTable()
	attrTable := dict.AttributeTable()
	strTable := dict.StringTable()

	for _, sample := range prof.Samples().All() {
		si := int(sample.StackIndex())
		if si < 0 || si >= stackTable.Len() {
			continue
		}
		stack := stackTable.At(si)
		for _, li := range stack.LocationIndices().All() {
			if int(li) < 0 || int(li) >= locTable.Len() {
				continue
			}
			loc := locTable.At(int(li))
			ft := frameTypeForLocation(strTable, attrTable, loc)
			if ft == "" {
				continue
			}
			out[ft]++
		}
	}
	return out
}

func frameTypeForLocation(strTable pcommon.StringSlice, attrTable pprofile.KeyValueAndUnitSlice, loc pprofile.Location) string {
	for _, ai := range loc.AttributeIndices().All() {
		if int(ai) < 0 || int(ai) >= attrTable.Len() {
			continue
		}
		attr := attrTable.At(int(ai))
		key := strAt(strTable, int(attr.KeyStrindex()))
		if key != profileFrameTypeKey {
			continue
		}
		return attr.Value().Str()
	}
	return ""
}

func strAt(table pcommon.StringSlice, idx int) string {
	if idx < 0 || idx >= table.Len() {
		return ""
	}
	return table.At(idx)
}

func safeUint64ToInt64(v uint64) int64 {
	if v > uint64(1<<63-1) {
		return 1<<63 - 1
	}
	return int64(v)
}
