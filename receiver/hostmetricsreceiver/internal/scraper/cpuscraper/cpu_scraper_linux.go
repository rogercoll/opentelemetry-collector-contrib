// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package cpuscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper"

import (
	"github.com/prometheus/procfs"
	"github.com/shirou/gopsutil/v4/cpu"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/metadata_legacy"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/ucal"
)

func (s *cpuScraper) recordCPUTimeStateDataPoints(now pcommon.Timestamp, cpuTime cpu.TimesStat) {
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.User, cpuTime.CPU, metadata.AttributeCPUModeUser)
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.System, cpuTime.CPU, metadata.AttributeCPUModeSystem)
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Idle, cpuTime.CPU, metadata.AttributeCPUModeIdle)
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Irq, cpuTime.CPU, metadata.AttributeCPUModeInterrupt)
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Nice, cpuTime.CPU, metadata.AttributeCPUModeNice)
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Softirq, cpuTime.CPU, metadata.AttributeCPUModeSoftirq)
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Steal, cpuTime.CPU, metadata.AttributeCPUModeSteal)
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Iowait, cpuTime.CPU, metadata.AttributeCPUModeWait)

	s.mb_legacy.RecordSystemCPUTimeDataPoint(now, cpuTime.User, cpuTime.CPU, metadata_legacy.AttributeStateUser)
	s.mb_legacy.RecordSystemCPUTimeDataPoint(now, cpuTime.System, cpuTime.CPU, metadata_legacy.AttributeStateSystem)
	s.mb_legacy.RecordSystemCPUTimeDataPoint(now, cpuTime.Idle, cpuTime.CPU, metadata_legacy.AttributeStateIdle)
	s.mb_legacy.RecordSystemCPUTimeDataPoint(now, cpuTime.Irq, cpuTime.CPU, metadata_legacy.AttributeStateInterrupt)
	s.mb_legacy.RecordSystemCPUTimeDataPoint(now, cpuTime.Nice, cpuTime.CPU, metadata_legacy.AttributeStateNice)
	s.mb_legacy.RecordSystemCPUTimeDataPoint(now, cpuTime.Softirq, cpuTime.CPU, metadata_legacy.AttributeStateSoftirq)
	s.mb_legacy.RecordSystemCPUTimeDataPoint(now, cpuTime.Steal, cpuTime.CPU, metadata_legacy.AttributeStateSteal)
	s.mb_legacy.RecordSystemCPUTimeDataPoint(now, cpuTime.Iowait, cpuTime.CPU, metadata_legacy.AttributeStateWait)
}

func (s *cpuScraper) recordCPUUtilization(now pcommon.Timestamp, cpuUtilization ucal.CPUUtilization) {
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.User, cpuUtilization.CPU, metadata.AttributeCPUModeUser)
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.System, cpuUtilization.CPU, metadata.AttributeCPUModeSystem)
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Idle, cpuUtilization.CPU, metadata.AttributeCPUModeIdle)
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Irq, cpuUtilization.CPU, metadata.AttributeCPUModeInterrupt)
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Nice, cpuUtilization.CPU, metadata.AttributeCPUModeNice)
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Softirq, cpuUtilization.CPU, metadata.AttributeCPUModeSoftirq)
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Steal, cpuUtilization.CPU, metadata.AttributeCPUModeSteal)
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Iowait, cpuUtilization.CPU, metadata.AttributeCPUModeWait)

	s.mb_legacy.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.User, cpuUtilization.CPU, metadata_legacy.AttributeStateUser)
	s.mb_legacy.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.System, cpuUtilization.CPU, metadata_legacy.AttributeStateSystem)
	s.mb_legacy.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Idle, cpuUtilization.CPU, metadata_legacy.AttributeStateIdle)
	s.mb_legacy.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Irq, cpuUtilization.CPU, metadata_legacy.AttributeStateInterrupt)
	s.mb_legacy.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Nice, cpuUtilization.CPU, metadata_legacy.AttributeStateNice)
	s.mb_legacy.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Softirq, cpuUtilization.CPU, metadata_legacy.AttributeStateSoftirq)
	s.mb_legacy.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Steal, cpuUtilization.CPU, metadata_legacy.AttributeStateSteal)
	s.mb_legacy.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Iowait, cpuUtilization.CPU, metadata_legacy.AttributeStateWait)
}

func (*cpuScraper) getCPUInfo() ([]cpuInfo, error) {
	var cpuInfos []cpuInfo
	fs, err := procfs.NewDefaultFS()
	if err != nil {
		return nil, scrapererror.NewPartialScrapeError(err, metricsLen)
	}
	cInf, err := fs.CPUInfo()
	if err != nil {
		return nil, scrapererror.NewPartialScrapeError(err, metricsLen)
	}
	for i := range cInf {
		cInfo := &cInf[i]
		c := cpuInfo{
			frequency: cInfo.CPUMHz,
			processor: cInfo.Processor,
		}
		cpuInfos = append(cpuInfos, c)
	}
	return cpuInfos, nil
}
