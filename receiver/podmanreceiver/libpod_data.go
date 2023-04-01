// Copyright The OpenTelemetry Authors
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

package podmanreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver"

import (
	"github.com/containers/podman/v4/libpod/define"
)

type container struct {
	AutoRemove bool
	Command    []string
	Created    string
	CreatedAt  string
	ExitCode   int
	Exited     bool
	ExitedAt   int
	ID         string
	Image      string
	ImageID    string
	IsInfra    bool
	Labels     map[string]string
	Mounts     []string
	Names      []string
	Namespaces map[string]string
	Networks   []string
	Pid        int
	Pod        string
	PodName    string
	Ports      []map[string]interface{}
	Size       map[string]string
	StartedAt  int
	State      string
	Status     string
}

type event struct {
	ID     string
	Status string
}

type containerStatsReportError struct {
	Cause    string
	Message  string
	Response int64
}

type containerStatsReport struct {
	Error containerStatsReportError
	Stats []define.ContainerStats
}
