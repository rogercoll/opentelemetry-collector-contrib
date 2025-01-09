// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupreceiver // import "go.opentelemetry.io/collector/receiver/nopreceiver"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

// NewFactory returns a receiver.Factory that constructs nop receivers.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		component.MustNewType("group"),
		createDefaultReceiverConfig,
		receiver.WithTraces(createTraces, component.StabilityLevelDevelopment),
		receiver.WithMetrics(createMetrics, component.StabilityLevelDevelopment),
		receiver.WithLogs(createLogs, component.StabilityLevelDevelopment))
}

func createDefaultReceiverConfig() component.Config {
	return &Config{
		cfg: make(map[component.ID]map[string]any),
	}
}

func createTraces(_ context.Context, set receiver.Settings, conf component.Config, _ consumer.Traces) (receiver.Traces, error) {
	return newXCreator(set, conf, pipeline.SignalTraces), nil
}

func createMetrics(_ context.Context, set receiver.Settings, conf component.Config, _ consumer.Metrics) (receiver.Metrics, error) {
	return newXCreator(set, conf, pipeline.SignalMetrics), nil
}

func createLogs(_ context.Context, set receiver.Settings, conf component.Config, _ consumer.Logs) (receiver.Logs, error) {
	return newXCreator(set, conf, pipeline.SignalLogs), nil
}

func newXCreator(settings receiver.Settings, cfg component.Config, signal pipeline.Signal) *xcreator {
	conf := cfg.(*Config)
	return &xcreator{
		signal: signal,
		cfg:    conf,
		logger: settings.Logger,
	}
}

// host is an interface that the component.Host passed to receivercreator's Start function must implement
type host interface {
	component.Host
	AddComponent(pipelineID pipeline.ID, kind component.Kind, compID component.ID, conf component.Config) error
	RemoveComponent(pipelineID pipeline.ID, kind component.Kind, compID component.ID) error
	GetFactory(component.Kind, component.Type) component.Factory
	InstanceID() *componentstatus.InstanceID
}

func (x *xcreator) Start(ctx context.Context, h component.Host) error {
	rcHost, ok := h.(host)
	if !ok {
		return errors.New("the receivercreator is not compatible with the provided component.host")
	}

	x.h = rcHost
	x.logger.Warn(fmt.Sprintf("Group receiver pipelines: %#v", x.h.InstanceID()))

	var merr error
	startAllPipelines := func(pipelineID pipeline.ID) bool {
		for compID, compConf := range x.cfg.cfg {
			factory := rcHost.GetFactory(component.KindReceiver, compID.Type())
			if factory == nil {
				merr = multierr.Append(merr, fmt.Errorf("unable to lookup factory for receiver %s:%q", compID.Type().String(), compID.String()))
				continue
			}
			conf := factory.CreateDefaultConfig()
			templatedConfig := confmap.NewFromStringMap(compConf)
			err := templatedConfig.Unmarshal(conf)
			if err != nil {
				x.logger.Error(fmt.Sprintf("merge conf", err.Error()))
				merr = multierr.Append(merr, err)
				continue
			}
			x.logger.Warn(fmt.Sprintf("Group receiver, adding component: %#v, with config: %#v", compID.String(), compConf))
			err = rcHost.AddComponent(pipelineID, component.KindReceiver, compID, conf)
			if err != nil {
				merr = multierr.Append(merr, err)
				continue
			}
		}
		return true
	}

	x.h.InstanceID().AllPipelineIDs(startAllPipelines)

	return merr
}

func (x *xcreator) Shutdown(context.Context) error {
	rcHost, ok := x.h.(host)
	if !ok {
		return errors.New("the receivercreator is not compatible with the provided component.host")
	}

	// var merr error
	stopAllPipelines := func(pipelineID pipeline.ID) bool {
		for compID := range x.cfg.cfg {
			x.logger.Info("Group stopping receiver", zap.String("recvID", compID.String()))
			err := rcHost.RemoveComponent(pipelineID, component.KindReceiver, compID)
			if err != nil {
				x.logger.Info(fmt.Sprintf("Xcreator error on removing receiver: %s", err.Error()))
				continue
			}
		}
		return true
	}

	x.h.InstanceID().AllPipelineIDs(stopAllPipelines)
	return nil
}

type xcreator struct {
	signal pipeline.Signal
	h      host
	cfg    *Config
	logger *zap.Logger
}
