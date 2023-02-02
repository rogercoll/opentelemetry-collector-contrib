package newrelicobserver

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
	"time"
)

var _ extension.Extension = (*newrelicHostObserver)(nil)
var _ observer.Observable = (*newrelicHostObserver)(nil)

type newrelicHostObserver struct {
	extension.Extension
	*observer.EndpointsWatcher
	logger *zap.Logger
	config *Config
}

func newObserver(logger *zap.Logger, config *Config) (extension.Extension, error) {
	d := &newrelicHostObserver{
		logger: logger, config: config,
	}
	d.EndpointsWatcher = observer.NewEndpointsWatcher(d, time.Second, logger)
	return d, nil
}

// Extension interface
func (h *newrelicHostObserver) Start(context.Context, component.Host) error {
	// go async func to gather NR guids
	return nil
}

// Extension interface
func (h *newrelicHostObserver) Shutdown(context.Context) error {
	//h.StopListAndWatch()
	return nil
}

func (n *newrelicHostObserver) getGuids() []string {
	return []string{
		"my_super_testing_guid_FFFFFF",
	}
}

func (n *newrelicHostObserver) ListEndpoints() []observer.Endpoint {
	var endpoints []observer.Endpoint
	for _, guid := range n.getGuids() {
		endpoints = append(endpoints, n.guidToEndpoint(guid))
	}
	return endpoints
}

func (n *newrelicHostObserver) guidToEndpoint(guid string) observer.Endpoint {
	return observer.Endpoint{
		ID:     observer.EndpointID(guid),
		Target: "localhost:8080",
		Details: &observer.Container{
			Name: guid,
		},
	}
}
