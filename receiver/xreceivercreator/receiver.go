// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xreceivercreator // import "go.opentelemetry.io/collector/receiver/nopreceiver"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/xreceivercreator/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.uber.org/zap"
)

// NewFactory returns a receiver.Factory that constructs nop receivers.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		func() component.Config { return &struct{}{} },
		receiver.WithTraces(createTraces, metadata.TracesStability),
		receiver.WithMetrics(createMetrics, metadata.MetricsStability),
		receiver.WithLogs(createLogs, metadata.LogsStability))
}

func createTraces(_ context.Context, set receiver.Settings, _ component.Config, _ consumer.Traces) (receiver.Traces, error) {
	return newXCreator(set), nil
}

func createMetrics(_ context.Context, set receiver.Settings, _ component.Config, _ consumer.Metrics) (receiver.Metrics, error) {
	return newXCreator(set), nil
}

func createLogs(_ context.Context, set receiver.Settings, _ component.Config, _ consumer.Logs) (receiver.Logs, error) {
	return newXCreator(set), nil
}

func newXCreator(settings receiver.Settings) *xcreator {
	return &xcreator{
		logger: settings.Logger,
	}
}

// host is an interface that the component.Host passed to receivercreator's Start function must implement
type host interface {
	component.Host
	AddComponent(pipelineID pipeline.ID, kind component.Kind, compID component.ID, conf component.Config) error
	RemoveComponent(kind component.Kind, compID component.ID) error
}

// TODO: It always create an otlp receiver, it should switch to a template
// provider instead
func (x *xcreator) Start(ctx context.Context, h component.Host) error {
	x.logger.Warn("Creating (sub) receiver")
	rcHost, ok := h.(host)
	if !ok {
		return errors.New("the receivercreator is not compatible with the provided component.host")
	}

	// OTLP receiver
	componentID := component.NewID(otlpreceiver.NewFactory().Type())
	cfg := otlpreceiver.NewFactory().CreateDefaultConfig()
	x.logger.Warn(fmt.Sprintf("(Sub)receiver config: %#v", cfg))

	err := rcHost.AddComponent(pipeline.NewID(pipeline.SignalMetrics), component.KindReceiver, componentID, cfg)
	if err != nil {
		x.logger.Error(fmt.Sprintf("Xcreator error on adding receiver: %s", err.Error()))
		return err
	}
	x.logger.Info(fmt.Sprintf("Xcreator no error on adding receiver"))
	// TODO: This is just for a quick test of the shutdown
	go func(id component.ID) {
		time.Sleep(35 * time.Second)
		x.logger.Info("Xcreator stopping receiver: %s", zap.String("recvID", id.String()))
		err := rcHost.RemoveComponent(component.KindReceiver, id)
		if err != nil {
			x.logger.Error(fmt.Sprintf("Xcreator error on removing receiver: %s", err.Error()))
		}
	}(componentID)

	// Dockerstats receiver
	// componentID = component.NewID(dockerstatsreceiver.NewFactory().Type())
	// cfg = dockerstatsreceiver.NewFactory().CreateDefaultConfig()
	// x.logger.Warn(fmt.Sprintf("(Sub)receiver config: %#v", cfg))
	//
	// err = rcHost.AddComponent(pipeline.NewID(pipeline.SignalMetrics), component.KindReceiver, componentID, cfg)
	// if err != nil {
	// 	x.logger.Error(fmt.Sprintf("Xcreator error on adding receiver: %s", err.Error()))
	// 	return err
	// }
	// x.logger.Info(fmt.Sprintf("Xcreator no error on adding receiver"))

	// Attributes processor (attributes/dynamic)
	componentID = component.NewIDWithName(attributesprocessor.NewFactory().Type(), "dynamic")
	cfg = attributesprocessor.NewFactory().CreateDefaultConfig()
	cfg2 := cfg.(*attributesprocessor.Config)
	cfg2.Settings = attraction.Settings{Actions: []attraction.ActionKeyValue{
		{Key: "dynamic", Value: "top", Action: "insert"},
	}}
	cfg = cfg2

	x.logger.Warn(fmt.Sprintf("(Sub)receiver config: %#v", cfg))

	err = rcHost.AddComponent(pipeline.NewID(pipeline.SignalMetrics), component.KindProcessor, componentID, cfg)
	if err != nil {
		x.logger.Error(fmt.Sprintf("Xcreator error on adding processor: %s", err.Error()))
		return err
	}
	x.logger.Info(fmt.Sprintf("Xcreator no error on adding processor"))

	// Resource detection
	componentID = component.NewIDWithName(resourceprocessor.NewFactory().Type(), "dynamic")
	cfg = resourceprocessor.NewFactory().CreateDefaultConfig()
	cfg3 := cfg.(*resourceprocessor.Config)
	cfg3.AttributesActions = []attraction.ActionKeyValue{
		{Key: "dynamic", Value: "top", Action: "insert"},
		{Key: "dynamic2", Value: true, Action: "insert"},
	}
	cfg = cfg3

	x.logger.Warn(fmt.Sprintf("(Sub)receiver config: %#v", cfg))

	err = rcHost.AddComponent(pipeline.NewID(pipeline.SignalMetrics), component.KindProcessor, componentID, cfg)
	if err != nil {
		x.logger.Error(fmt.Sprintf("Xcreator error on adding processor: %s", err.Error()))
		return err
	}
	x.logger.Info(fmt.Sprintf("Xcreator no error on adding processor"))

	go func(id component.ID) {
		time.Sleep(10 * time.Second)
		x.logger.Info("Xcreator stopping processor: %s", zap.String("procID", id.String()))
		err := rcHost.RemoveComponent(component.KindProcessor, id)
		if err != nil {
			x.logger.Error(fmt.Sprintf("Xcreator error on removing processor: %s", err.Error()))
		}
	}(componentID)

	return nil
}

type xcreator struct {
	logger *zap.Logger
	component.ShutdownFunc
}
