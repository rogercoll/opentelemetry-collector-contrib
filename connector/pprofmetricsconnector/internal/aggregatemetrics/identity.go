// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package aggregatemetrics implements in-memory merging of OTLP metrics before export,
// following the same stream-identity model as Elasticsearch profilingmetricsconnector
// (resource + scope + metric + datapoint attributes, without timestamp in the stream key).
package aggregatemetrics

import (
	"hash"
	"hash/fnv"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

// Resource identifies a resource by attribute hash.
type Resource struct {
	attrs [16]byte
}

// Hash implements hash for map keys.
func (r Resource) Hash() hash.Hash64 {
	sum := fnv.New64a()
	sum.Write(r.attrs[:])
	return sum
}

// OfResource builds a Resource identity.
func OfResource(r pcommon.Resource) Resource {
	return Resource{attrs: pdatautil.MapHash(r.Attributes())}
}

// Scope identifies an instrumentation scope within a resource.
type Scope struct {
	resource Resource
	name     string
	version  string
	attrs    [16]byte
}

// Hash implements hash for map keys.
func (s Scope) Hash() hash.Hash64 {
	sum := s.resource.Hash()
	sum.Write([]byte(s.name))
	sum.Write([]byte(s.version))
	sum.Write(s.attrs[:])
	return sum
}

// OfScope builds a Scope identity.
func OfScope(res Resource, scope pcommon.InstrumentationScope) Scope {
	return Scope{
		resource: res,
		name:     scope.Name(),
		version:  scope.Version(),
		attrs:    pdatautil.MapHash(scope.Attributes()),
	}
}

// Metric identifies a metric within a scope (name, unit, type, …).
type Metric struct {
	scope       Scope
	name        string
	unit        string
	ty          pmetric.MetricType
	monotonic   bool
	temporality pmetric.AggregationTemporality
}

// Hash implements hash for map keys.
func (m Metric) Hash() hash.Hash64 {
	sum := m.scope.Hash()
	sum.Write([]byte(m.name))
	sum.Write([]byte(m.unit))

	var mono byte
	if m.monotonic {
		mono = 1
	}
	sum.Write([]byte{byte(m.ty), mono, byte(m.temporality)})
	return sum
}

// OfMetric builds a Metric identity.
func OfMetric(scope Scope, m pmetric.Metric) Metric {
	id := Metric{
		scope: scope,
		name:  m.Name(),
		unit:  m.Unit(),
		ty:    m.Type(),
	}
	switch m.Type() {
	case pmetric.MetricTypeSum:
		sum := m.Sum()
		id.monotonic = sum.IsMonotonic()
		id.temporality = sum.AggregationTemporality()
	case pmetric.MetricTypeExponentialHistogram:
		exp := m.ExponentialHistogram()
		id.monotonic = true
		id.temporality = exp.AggregationTemporality()
	case pmetric.MetricTypeHistogram:
		hist := m.Histogram()
		id.monotonic = true
		id.temporality = hist.AggregationTemporality()
	}
	return id
}

// Stream identifies a metric time series: metric identity + datapoint attributes.
// Timestamp is intentionally not part of the stream key so duplicate observations
// in a flush window merge (same model as Elasticsearch profilingmetricsconnector).
type Stream struct {
	metric Metric
	attrs  [16]byte
}

// Hash implements hash for map keys.
func (s Stream) Hash() hash.Hash64 {
	sum := s.metric.Hash()
	sum.Write(s.attrs[:])
	return sum
}

// OfStream builds a stream key from a metric identity and a number data point.
func OfStream(m Metric, dp interface{ Attributes() pcommon.Map }) Stream {
	return Stream{metric: m, attrs: pdatautil.MapHash(dp.Attributes())}
}
