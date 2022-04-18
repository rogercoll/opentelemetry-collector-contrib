// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !windows
// +build !windows

package podmanreceiver

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

type MockClient struct {
	PingF   func(context.Context) error
	StatsF  func(context.Context, url.Values) ([]containerStats, error)
	ListF   func(context.Context, url.Values) ([]Container, error)
	EventsF func(context.Context, url.Values) (<-chan Event, <-chan error)
}

func (c *MockClient) ping(ctx context.Context) error {
	return c.PingF(ctx)
}

func (c *MockClient) stats(ctx context.Context, options url.Values) ([]containerStats, error) {
	return c.StatsF(ctx, options)
}

func (c *MockClient) list(ctx context.Context, options url.Values) ([]Container, error) {
	return c.ListF(ctx, options)
}

func (c *MockClient) events(ctx context.Context, options url.Values) (<-chan Event, <-chan error) {
	return c.EventsF(ctx, options)
}

var baseClient = MockClient{
	PingF: func(context.Context) error {
		return nil
	},
	StatsF: func(context.Context, url.Values) ([]containerStats, error) {
		return nil, nil
	},
	ListF: func(context.Context, url.Values) ([]Container, error) {
		return nil, nil
	},
	EventsF: func(context.Context, url.Values) (<-chan Event, <-chan error) {
		return nil, nil
	},
}

func tmpSock(t *testing.T) (net.Listener, string) {
	f, err := ioutil.TempFile(os.TempDir(), "testsock")
	if err != nil {
		t.Fatal(err)
	}
	addr := f.Name()
	os.Remove(addr)

	listener, err := net.Listen("unix", addr)
	if err != nil {
		t.Fatal(err)
	}

	return listener, addr
}

func TestWatchingTimeouts(t *testing.T) {
	listener, addr := tmpSock(t)
	defer listener.Close()
	defer os.Remove(addr)

	config := &Config{
		Endpoint: fmt.Sprintf("unix://%s", addr),
		Timeout:  50 * time.Millisecond,
	}

	client, err := newLibpodClient(zap.NewNop(), config)
	assert.Nil(t, err)

	cli := NewContainerScraper(client, zap.NewNop(), config)
	assert.NotNil(t, cli)

	expectedError := "context deadline exceeded"

	shouldHaveTaken := time.Now().Add(100 * time.Millisecond).UnixNano()

	err = cli.LoadContainerList(context.Background())
	require.Error(t, err)

	containers, err := cli.FetchContainerStats(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), expectedError)
	assert.Nil(t, containers)

	assert.GreaterOrEqual(
		t, time.Now().UnixNano(), shouldHaveTaken,
		"Client timeouts don't appear to have been exercised.",
	)
}

func TestStats(t *testing.T) {
	// stats sample
	statsExample := `{"Error":null,"Stats":[{"AvgCPU":42.04781177856639,"ContainerID":"e6af5805edae6c950003abd5451808b277b67077e400f0a6f69d01af116ef014","Name":"charming_sutherland","PerCPU":null,"CPU":42.04781177856639,"CPUNano":309165846000,"CPUSystemNano":54515674,"SystemNano":1650912926385978706,"MemUsage":27717632,"MemLimit":7942234112,"MemPerc":0.34899036730888044,"NetInput":430,"NetOutput":330,"BlockInput":0,"BlockOutput":0,"PIDs":118,"UpTime":309165846000,"Duration":309165846000}]}`

	listener, addr := tmpSock(t)
	defer listener.Close()
	defer os.Remove(addr)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/stats") {
			_, err := w.Write([]byte(statsExample))
			assert.NoError(t, err)
		} else {
			_, err := w.Write([]byte{})
			assert.NoError(t, err)
		}
	}))
	srv.Listener = listener
	srv.Start()
	defer srv.Close()

	config := &Config{
		Endpoint: fmt.Sprintf("unix://%s", addr),
		// default timeout
		Timeout: 5 * time.Second,
	}

	cli, err := newLibpodClient(zap.NewNop(), config)
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	expectedStats := containerStats{
		AvgCPU:        42.04781177856639,
		ContainerID:   "e6af5805edae6c950003abd5451808b277b67077e400f0a6f69d01af116ef014",
		Name:          "charming_sutherland",
		PerCPU:        nil,
		CPU:           42.04781177856639,
		CPUNano:       309165846000,
		CPUSystemNano: 54515674,
		SystemNano:    1650912926385978706,
		MemUsage:      27717632,
		MemLimit:      7942234112,
		MemPerc:       0.34899036730888044,
		NetInput:      430,
		NetOutput:     330,
		BlockInput:    0,
		BlockOutput:   0,
		PIDs:          118,
		UpTime:        309165846000 * time.Nanosecond,
		Duration:      309165846000,
	}

	stats, err := cli.stats(context.Background(), nil)
	assert.NoError(t, err)
	assert.Equal(t, expectedStats, stats[0])
}

func TestStatsError(t *testing.T) {
	// If the stats request fails, the API returns the following Error structure: https://docs.podman.io/en/latest/_static/api.html#operation/ContainersStatsAllLibpod
	// For example, if we query the stats with an invalid container ID, the API returns the following message
	statsError := `{"Error":{},"Stats":null}`

	listener, addr := tmpSock(t)
	defer listener.Close()
	defer os.Remove(addr)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/stats") {
			_, err := w.Write([]byte(statsError))
			assert.NoError(t, err)
		} else {
			_, err := w.Write([]byte{})
			assert.NoError(t, err)
		}
	}))
	srv.Listener = listener
	srv.Start()
	defer srv.Close()

	config := &Config{
		Endpoint: fmt.Sprintf("unix://%s", addr),
		// default timeout
		Timeout: 5 * time.Second,
	}

	cli, err := newLibpodClient(zap.NewNop(), config)
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	stats, err := cli.stats(context.Background(), nil)
	assert.Nil(t, stats)
	assert.EqualError(t, err, errNoStatsFound.Error())
}

// PODMAN TESTS

func TestEventLoopHandlesError(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(2) // confirm retry occurs

	listener, addr := tmpSock(t)
	defer listener.Close()
	defer os.Remove(addr)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/events") {
			wg.Done()
		}
		w.Write([]byte{})
	}))
	srv.Listener = listener
	srv.Start()
	defer srv.Close()

	observed, logs := observer.New(zapcore.WarnLevel)
	config := &Config{
		Endpoint: fmt.Sprintf("unix://%s", addr),
		Timeout:  50 * time.Millisecond,
	}

	client, err := newLibpodClient(zap.NewNop(), config)
	assert.Nil(t, err)

	cli := NewContainerScraper(client, zap.New(observed), config)
	assert.NotNil(t, cli)

	go cli.ContainerEventLoop(context.Background())

	assert.Eventually(t, func() bool {
		for _, l := range logs.All() {
			assert.Contains(t, l.Message, "Error watching podman container events")
			assert.Contains(t, l.ContextMap()["error"], "EOF")
		}
		return len(logs.All()) > 0
	}, 1*time.Second, 1*time.Millisecond, "failed to find desired error logs.")

	finished := make(chan struct{})
	go func() {
		defer close(finished)
		wg.Wait()
	}()
	select {
	case <-time.After(5 * time.Second):
		t.Fatal("failed to retry events endpoint after error")
	case <-finished:
		return
	}
}

func TestEventLoopHandles(t *testing.T) {
	eventChan := make(chan Event)
	errChan := make(chan error)

	eventClient := baseClient
	eventClient.EventsF = func(context.Context, url.Values) (<-chan Event, <-chan error) {
		return eventChan, errChan
	}
	eventClient.ListF = func(context.Context, url.Values) ([]Container, error) {
		return []Container{{
			Id: "c1",
		}}, nil
	}

	cli := NewContainerScraper(&eventClient, zap.NewNop(), &Config{})
	assert.NotNil(t, cli)

	assert.Equal(t, 0, len(cli.containers))

	go cli.ContainerEventLoop(context.Background())
	eventChan <- Event{ID: "c1", Status: "start"}

	assert.Eventually(t, func() bool {
		return assert.Equal(t, 1, len(cli.containers))
	}, 1*time.Second, 1*time.Millisecond, "failed to update containers list.")

	eventChan <- Event{ID: "c1", Status: "died"}

	assert.Eventually(t, func() bool {
		return assert.Equal(t, 0, len(cli.containers))
	}, 1*time.Second, 1*time.Millisecond, "failed to update containers list.")
}

func TestInspectAndPersistContainer(t *testing.T) {
	inspectClient := baseClient
	inspectClient.ListF = func(context.Context, url.Values) ([]Container, error) {
		return []Container{{
			Id: "c1",
		}}, nil
	}

	cli := NewContainerScraper(&inspectClient, zap.NewNop(), &Config{})
	assert.NotNil(t, cli)

	assert.Equal(t, 0, len(cli.containers))

	stats, ok := cli.InspectAndPersistContainer(context.Background(), "c1")
	assert.True(t, ok)
	assert.NotNil(t, stats)
	assert.Equal(t, 1, len(cli.containers))
}
