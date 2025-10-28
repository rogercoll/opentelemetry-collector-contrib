package profilingsessionsobserver

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/elastic/go-elasticsearch/v8"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

var (
	_ extension.Extension      = (*elasticsearchObserver)(nil)
	_ observer.Observable      = (*elasticsearchObserver)(nil)
	_ observer.EndpointDetails = (*ottlDetails)(nil)
)

type elasticsearchObserver struct {
	telemetry component.TelemetrySettings
	config    *Config
	client    *elasticsearch.Client

	sessionsFetcher *ElasticsearchFetcher
	eCtx            context.Context
	eCancelFn       context.CancelFunc
}

func newObserver(
	telemetry component.TelemetrySettings,
	config *Config,
) (extension.Extension, error) {
	o := &elasticsearchObserver{
		telemetry: telemetry,
		config:    config,
	}

	return o, nil
}

func (e *elasticsearchObserver) Start(ctx context.Context, host component.Host) error {
	e.eCtx, e.eCancelFn = context.WithCancel(ctx)
	var err error
	e.client, err = e.config.Source.Elasticsearch.ToClient(e.eCtx, host, e.telemetry)
	if err != nil {
		return err
	}

	e.sessionsFetcher = NewElasticsearchFetcher(e.client, 10*time.Second, e.telemetry.Logger)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		// ensure Go routine is scheduled
		wg.Done()
		if err := e.sessionsFetcher.Run(e.eCtx); err != nil {
			e.telemetry.Logger.Error(err.Error())
		}
	}()

	wg.Wait()

	return nil
}

func (e *elasticsearchObserver) Shutdown(context.Context) error {
	if e.eCancelFn != nil {
		e.eCancelFn()
	}
	return nil
}

// ListAndWatch provides initial state sync as well as change notification.
// notify. OnAdd will be called one or more times if there are endpoints discovered.
// (It would not be called if there are no endpoints present.) The endpoint synchronization
// happens asynchronously to this call.
func (e *elasticsearchObserver) ListAndWatch(notify observer.Notify) {
	go func() {
		for {

			ticker := time.NewTicker(10 * time.Second)
			select {
			case <-ticker.C:
				sessions, err := e.sessionsFetcher.Fetch(context.Background())
				if err != nil {
					e.telemetry.Logger.Error(err.Error())
					continue
				}
				for _, sess := range sessions {
					e.telemetry.Logger.Warn(fmt.Sprintf("fetched session: %#v\n", sess))
					go func(profilingSession Session) {
						sessDuration, err := time.ParseDuration(sess.Duration)
						if err != nil {
							e.telemetry.Logger.Error(err.Error())
							return
						}
						sessionID := rand.Text()
						notify.OnAdd([]observer.Endpoint{
							{
								ID:     observer.EndpointID(sessionID),
								Target: "not_used",
								Details: &ottlDetails{
									signal:        "profile",
									ottlstatement: resourcesToOTTL(sess.Resources),
								},
							},
						})
						sessTimer := time.NewTimer(sessDuration)
						select {
						case <-sessTimer.C:
						case <-e.eCtx.Done():
						}
						notify.OnRemove([]observer.Endpoint{
							{
								ID: observer.EndpointID(sessionID),
							},
						})
					}(sess)
				}

			case <-e.eCtx.Done():
				break
			}
		}
	}()
}

func resourcesToOTTL(resources map[string]string) string {
	if len(resources) == 0 {
		return `''`
	}

	var b strings.Builder
	b.WriteString(`'(`)
	i := 0
	for key, value := range resources {
		if i > 0 {
			b.WriteString(" and ")
		}
		fmt.Fprintf(&b, `resource.attributes["%s"] == "%s"`, key, value)
		i++
	}
	b.WriteString(`)'`)
	return b.String()
}

// Unsubscribe stops the previously registered Notify from receiving callback invocations.
func (k *elasticsearchObserver) Unsubscribe(notify observer.Notify) {}

type ottlDetails struct {
	// TODO: change to pipeline.Signal
	signal        string
	ottlstatement string
}

func (p *ottlDetails) Env() observer.EndpointEnv {
	return map[string]any{
		"signal": p.signal,
		"ottl":   p.ottlstatement,
	}
}

func (p *ottlDetails) Type() observer.EndpointType {
	return "ottlexpression"
}
