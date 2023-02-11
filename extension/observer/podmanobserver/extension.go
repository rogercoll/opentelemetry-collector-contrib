package podmanobserver

import (
	"context"
	"sync"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
)

const (
	defaultPodmanAPIVersion         = 1.22
	minimalRequiredPodmanAPIVersion = 1.22
)

var _ extension.Extension = (*podmanObserver)(nil)
var _ observer.Observable = (*podmanObserver)(nil)
var _ observer.EndpointsLister = (*podmanObserver)(nil)

type podmanObserver struct {
	*observer.EndpointsWatcher
	logger  *zap.Logger
	config  *Config
	once    *sync.Once
	cancel  func()
	pClient *podman.Client
}

// newObserver creates a new podman observer extension.
func newObserver(logger *zap.Logger, config *Config) (extension.Extension, error) {
	p := &podmanObserver{
		logger: logger,
		config: config,
		once:   &sync.Once{},
	}
	p.EndpointsWatcher = observer.NewEndpointsWatcher(p, time.Second, logger)
	return p, nil
}

// Start will instantiate required components needed by the Podman observer
func (p *podmanObserver) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (p *podmanObserver) Shutdown(ctx context.Context) error {
	p.cancel()
	return nil
}

func (p *podmanObserver) ListEndpoints() []observer.Endpoint {
	return nil
}
