// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build linux
// +build linux

package memoryscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper"

import (
	"github.com/shirou/gopsutil/v3/mem"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper/internal/metadata"
)

const memStatesLen = 6

func appendMemoryUsageStateDataPoints(idps pdata.NumberDataPointSlice, now pdata.Timestamp, memInfo *mem.VirtualMemoryStat) {
	initializeMemoryUsageDataPoint(idps.AppendEmpty(), now, metadata.AttributeState.Used, int64(memInfo.Used))
	initializeMemoryUsageDataPoint(idps.AppendEmpty(), now, metadata.AttributeState.Free, int64(memInfo.Free))
	initializeMemoryUsageDataPoint(idps.AppendEmpty(), now, metadata.AttributeState.Buffered, int64(memInfo.Buffers))
	initializeMemoryUsageDataPoint(idps.AppendEmpty(), now, metadata.AttributeState.Cached, int64(memInfo.Cached))
	initializeMemoryUsageDataPoint(idps.AppendEmpty(), now, metadata.AttributeState.SlabReclaimable, int64(memInfo.Sreclaimable))
	initializeMemoryUsageDataPoint(idps.AppendEmpty(), now, metadata.AttributeState.SlabUnreclaimable, int64(memInfo.Sunreclaim))
}
func appendMemoryUtilizationStateDataPoints(idps pdata.NumberDataPointSlice, now pdata.Timestamp, memInfo *mem.VirtualMemoryStat) {
	initializeMemoryUtilizationDataPoint(idps.AppendEmpty(), now, metadata.AttributeState.Used, float64(memInfo.Used)/float64(memInfo.Total))
	initializeMemoryUtilizationDataPoint(idps.AppendEmpty(), now, metadata.AttributeState.Free, float64(memInfo.Free)/float64(memInfo.Total))
	initializeMemoryUtilizationDataPoint(idps.AppendEmpty(), now, metadata.AttributeState.Buffered, float64(memInfo.Buffers)/float64(memInfo.Total))
	initializeMemoryUtilizationDataPoint(idps.AppendEmpty(), now, metadata.AttributeState.Cached, float64(memInfo.Cached)/float64(memInfo.Total))
	initializeMemoryUtilizationDataPoint(idps.AppendEmpty(), now, metadata.AttributeState.SlabReclaimable, float64(memInfo.Sreclaimable)/float64(memInfo.Total))
	initializeMemoryUtilizationDataPoint(idps.AppendEmpty(), now, metadata.AttributeState.SlabUnreclaimable, float64(memInfo.Sunreclaim)/float64(memInfo.Total))
}
