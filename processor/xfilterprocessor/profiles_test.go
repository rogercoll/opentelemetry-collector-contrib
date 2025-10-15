package xfilterprocessor

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/xfilterprocessor/internal/metadata"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/collector/processor/processortest"
)

type profilingDetails struct {
	profiles []string
}

func (p *profilingDetails) Env() observer.EndpointEnv {
	return map[string]any{
		"profiles": p.profiles,
	}
}

func (p *profilingDetails) Type() observer.EndpointType {
	return "filterexpression"
}

type observerMock struct {
	toNotify observer.Notify
}

func (o *observerMock) onAdd(endpoint profilingDetails) {
	fmt.Println(endpoint.profiles)
	o.toNotify.OnAdd([]observer.Endpoint{
		{
			ID:      "test",
			Target:  "not used",
			Details: &endpoint,
		},
	})
}

// func (o *observerMock) onRe(endpoint []observer.Endpoint) {}

func (o *observerMock) ListAndWatch(notify observer.Notify) {
	o.toNotify = notify
}

// Unsubscribe stops the previously registered Notify from receiving callback invocations.
func (o *observerMock) Unsubscribe(notify observer.Notify) {}

func (o *observerMock) Start(context.Context, component.Host) error {
	return nil
}

func (o *observerMock) Shutdown(context.Context) error {
	return nil
}

var _ component.Host = (*nopHost)(nil)

// nopHost mocks a [component.Host] for testing purposes.
type nopHost struct{}

// NewNopHost returns a [component.Host] that returns empty values
// from method calls. This host is intended to be used in tests
// where a bare-minimum host is desired.
func NewNopHost() component.Host {
	return &nopHost{}
}

// GetExtensions returns a `nil` extensions map.
func (nh *nopHost) GetExtensions() map[component.ID]component.Component {
	val := sync.OnceValue(func() map[component.ID]component.Component {
		return map[component.ID]component.Component{
			component.MustNewID("observerMock"): &observerMock{},
		}
	})
	return val()
}

func TestFilterProfileProcessorWithRemoteOTTL(t *testing.T) {
	tests := []struct {
		name             string
		conditions       []string
		filterEverything bool
		want             func(pprofile.Profiles)
		errorMode        ottl.ErrorMode
	}{
		{
			name: "drop profiles",
			conditions: []string{
				`original_payload_format == "legacy"`,
			},
			want: func(ld pprofile.Profiles) {
				ld.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().RemoveIf(func(profile pprofile.Profile) bool {
					return profile.OriginalPayloadFormat() == "legacy"
				})
				ld.ResourceProfiles().At(0).ScopeProfiles().At(1).Profiles().RemoveIf(func(profile pprofile.Profile) bool {
					return profile.OriginalPayloadFormat() == "legacy"
				})
			},
			errorMode: ottl.IgnoreError,
		},
		{
			name: "drop everything by dropping all profiles",
			conditions: []string{
				`IsMatch(original_payload_format, ".*legacy")`,
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "multiple conditions",
			conditions: []string{
				`IsMatch(original_payload_format, "wrong name")`,
				`IsMatch(original_payload_format, ".*legacy")`,
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "with error conditions",
			conditions: []string{
				`Substring("", 0, 100) == "test"`,
			},
			want:      func(_ pprofile.Profiles) {},
			errorMode: ottl.IgnoreError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{WatchObservers: []component.ID{component.MustNewID("observerMock")}, profileFunctions: defaultProfileFunctionsMap()}
			processor, err := newFilterProfilesProcessor(processortest.NewNopSettings(metadata.Type), cfg)
			assert.NoError(t, err)
			host := nopHost{}

			assert.NoError(t, processor.start(t.Context(), &host))

			processor.observers[component.MustNewID("observerMock")].(*observerMock).onAdd(profilingDetails{
				profiles: tt.conditions,
			})

			got, err := processor.processProfiles(t.Context(), constructProfiles())

			if tt.filterEverything {
				assert.Equal(t, processorhelper.ErrSkipProcessingData, err)
			} else {
				exTd := constructProfiles()
				tt.want(exTd)
				assert.Equal(t, exTd, got)
			}
		})
	}
}

func constructProfiles() pprofile.Profiles {
	td := pprofile.NewProfiles()
	rs0 := td.ResourceProfiles().AppendEmpty()
	rs0.Resource().Attributes().PutStr("host.name", "localhost")
	rs0ils0 := rs0.ScopeProfiles().AppendEmpty()
	rs0ils0.Scope().SetName("scope1")
	fillProfileOne(rs0ils0.Profiles().AppendEmpty())
	fillProfileTwo(rs0ils0.Profiles().AppendEmpty())
	rs0ils1 := rs0.ScopeProfiles().AppendEmpty()
	rs0ils1.Scope().SetName("scope2")
	fillProfileOne(rs0ils1.Profiles().AppendEmpty())
	fillProfileTwo(rs0ils1.Profiles().AppendEmpty())
	return td
}

func fillProfileOne(profile pprofile.Profile) {
	profile.SetOriginalPayloadFormat("legacy")
	profile.Sample().AppendEmpty()
}

func fillProfileTwo(profile pprofile.Profile) {
	profile.SetOriginalPayloadFormat("non-legacy")
	profile.Sample().AppendEmpty()
}
