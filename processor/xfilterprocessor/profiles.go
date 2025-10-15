package xfilterprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"
)

var _ observer.Notify = (*filterExpression)(nil)

type filterProfileProcessor struct {
	logger *zap.Logger
	// WatchObservers are the extensions to listen to endpoints from.
	watchObservers []component.ID `mapstructure:"watch_observers"`
	observers      map[component.ID]observer.Observable

	expression *filterExpression
}

type filterExpression struct {
	mu sync.RWMutex

	skipExpr expr.BoolExpr[ottlprofile.TransformContext]

	// dynamic endpoint conditions
	endpointConditions map[observer.EndpointID][]string
	conditionsLen      int
	// expression factory function
	exprFn func([]string) (expr.BoolExpr[ottlprofile.TransformContext], error)
}

func newFilterProfilesProcessor(set processor.Settings, cfg *Config) (*filterProfileProcessor, error) {
	fpp := &filterProfileProcessor{
		logger:         set.Logger,
		watchObservers: cfg.WatchObservers,
		expression: &filterExpression{
			endpointConditions: make(map[observer.EndpointID][]string),
			exprFn: func(conditions []string) (expr.BoolExpr[ottlprofile.TransformContext], error) {
				allConditions := append(cfg.Profiles.ProfileConditions, conditions...)
				return filterottl.NewBoolExprForProfile(allConditions, cfg.profileFunctions, cfg.ErrorMode, set.TelemetrySettings)
			},
		},
		observers: make(map[component.ID]observer.Observable),
	}

	// should we let the user define manual conditions?
	if cfg.Profiles.ProfileConditions != nil {
		var errBoolExpr error
		fpp.expression.skipExpr, errBoolExpr = fpp.expression.exprFn(nil)
		if errBoolExpr != nil {
			return nil, errBoolExpr
		}
	}

	return fpp, nil
}

// processProfiles filters the given samples of a profiles based off the filterSampleProcessor's filters.
func (fpp *filterProfileProcessor) processProfiles(ctx context.Context, pd pprofile.Profiles) (pprofile.Profiles, error) {
	fpp.expression.mu.RLock()
	defer fpp.expression.mu.RUnlock()

	if fpp.expression.skipExpr == nil {
		return pd, nil
	}

	dic := pd.Dictionary()

	var errors error
	pd.ResourceProfiles().RemoveIf(func(rp pprofile.ResourceProfiles) bool {
		resource := rp.Resource()
		rp.ScopeProfiles().RemoveIf(func(sp pprofile.ScopeProfiles) bool {
			scope := sp.Scope()
			sp.Profiles().RemoveIf(func(profile pprofile.Profile) bool {
				if fpp.expression.skipExpr != nil {
					skip, err := fpp.expression.skipExpr.Eval(ctx, ottlprofile.NewTransformContext(profile, dic, scope, resource, sp, rp))
					if err != nil {
						errors = multierr.Append(errors, err)
						return false
					}
					if skip {
						return true
					}
				}
				return false
			})
			return sp.Profiles().Len() == 0
		})
		return rp.ScopeProfiles().Len() == 0
	})

	if errors != nil {
		fpp.logger.Error("failed processing profiles", zap.Error(errors))
		return pd, errors
	}
	if pd.ResourceProfiles().Len() == 0 {
		return pd, processorhelper.ErrSkipProcessingData
	}
	return pd, nil
}

// host is an interface that the component.Host passed to receivercreator's Start function must implement
type host interface {
	component.Host
}

func (fpp *filterProfileProcessor) start(ctx context.Context, h component.Host) error {
	rcHost, ok := h.(host)
	if !ok {
		return errors.New("the xfilterprocessor is not compatible with the provided component.host")
	}
	for _, watchObserver := range fpp.watchObservers {
		fpp.logger.Warn(fmt.Sprintf("found observer %s", watchObserver.String()))
		// TODO: open issue for receiver creator? what if extension is
		// not part of the collector distribution? silently wait
		for cid, ext := range rcHost.GetExtensions() {
			if cid != watchObserver {
				continue
			}

			obs, ok := ext.(observer.Observable)
			if !ok {
				return fmt.Errorf("extension %q in watch_observers is not an observer", watchObserver.String())
			}
			fpp.observers[watchObserver] = obs
			fpp.observers[watchObserver].ListAndWatch(fpp.expression)
		}
	}
	return nil
}

// TODO: move to contrib observer endpoint package
const (
	// ProfilingSessionType is a profiling session endpoint.
	FilterExpressionType observer.EndpointType = "filterexpression"
)

func (fe *filterExpression) ID() observer.NotifyID {
	return ""
}

func (fe *filterExpression) onchange(changed []observer.Endpoint) {
	for _, endpoint := range changed {
		if endpoint.Details.Type() != FilterExpressionType {
			// TODO: add log line
			continue
		}
		filterEnvs := endpoint.Details.Env()
		profilesConditions, ok := filterEnvs["profiles"].([]string)
		if !ok {
			// TODO: add log line
			continue
		}
		fe.mu.Lock()
		fmt.Println(profilesConditions)

		fe.endpointConditions[endpoint.ID] = profilesConditions
		fe.conditionsLen += len(profilesConditions)

		allConditions := make([]string, 0)
		for _, conditions := range fe.endpointConditions {
			allConditions = append(allConditions, conditions...)
		}

		skipExpr, err := fe.exprFn(allConditions)
		if err != nil {
			fmt.Printf("error while creating new expr: %#v\n", err.Error())
			fe.mu.Unlock()
			// TODO: add log line
			continue
		}

		fe.skipExpr = skipExpr

		fe.mu.Unlock()
	}
}

// OnAdd is called once or more initially for state sync as well as when further endpoints are added.
func (fe *filterExpression) OnAdd(added []observer.Endpoint) {
	fe.onchange(added)
}

// OnRemove is called when one or more endpoints are removed.
func (fe *filterExpression) OnRemove(removed []observer.Endpoint) {
	for _, endpoint := range removed {
		if endpoint.Details.Type() != FilterExpressionType {
			// TODO: add log line
			continue
		}
		fe.mu.Lock()

		fe.conditionsLen -= len(fe.endpointConditions[endpoint.ID])
		delete(fe.endpointConditions, endpoint.ID)

		allConditions := make([]string, fe.conditionsLen)
		for _, conditions := range fe.endpointConditions {
			allConditions = append(allConditions, conditions...)
		}

		skipExpr, err := fe.exprFn(allConditions)
		if err != nil {
			fe.mu.Unlock()
			// TODO: add log line
			continue
		}

		fe.skipExpr = skipExpr

		fe.mu.Unlock()
	}
}

// OnChange is called when one or more endpoints are modified but the identity is not changed
// (e.g. labels).
func (fe *filterExpression) OnChange(changed []observer.Endpoint) {
	fe.onchange(changed)
}
