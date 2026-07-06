// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializeprofiles // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer/serializeprofiles"

import (
	"encoding/json"
	"strings"
	"time"

	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// EcsVersionString is the value for the `ecs.version` metrics field.
// It is relatively arbitrary and currently has no consumer.
// APM server is using 1.12.0. We stick with it as well.
const EcsVersionString = "1.12.0"

// EcsVersion is a struct to hold the `ecs.version` metrics field.
// Used as a helper in ES index struct types.
type EcsVersion struct {
	V string `json:"ecs.version"`
}

// StackPayload represents a single [StackTraceEvent], with a [StackTrace], a
// map of [StackFrames] and a map of [ExeMetadata] that have been serialized,
// and need to be ingested into ES.
type StackPayload struct {
	StackTraceEvent StackTraceEvent
	StackTrace      StackTrace
	StackFrames     []StackFrame
	Executables     []ExeMetadata

	ResourceAttrs ResourceData

	UnsymbolizedLeafFrames  []UnsymbolizedLeafFrame
	UnsymbolizedExecutables []UnsymbolizedExecutable
}

// StackTraceEvent represents a stacktrace event serializable into ES.
// The json field names need to be case-sensitively equal to the fields defined
// in the schema mapping.
type StackTraceEvent struct {
	EcsVersion
	TimeStamp    unixTime64 `json:"@timestamp"`
	HostID       string     `json:"host.id"`
	StackTraceID string     `json:"Stacktrace.id"` // 128-bit hash in binary form

	// Event-specific metadata
	PodName          string `json:"orchestrator.resource.name,omitempty"`
	ContainerID      string `json:"container.id,omitempty"`
	ContainerName    string `json:"container.name,omitempty"`
	K8sNamespaceName string `json:"k8s.namespace.name,omitempty"`
	ThreadName       string `json:"process.thread.name"`
	ExecutableName   string `json:"process.executable.name"`
	ServiceName      string `json:"service.name,omitempty"`
	Frequency        int64  `json:"Stacktrace.sampling_frequency"`
	Count            uint16 `json:"Stacktrace.count"`
	ProjectID        uint32 `json:"profiling.project.id,omitempty"`
	HostName         string `json:"host.name,omitempty"`
}

// StackTrace represents a stacktrace serializable into the stacktraces index.
// DocID should be the base64-encoded Stacktrace ID.
type StackTrace struct {
	EcsVersion
	DocID    string `json:"-"`
	FrameIDs string `json:"Stacktrace.frame.ids"`
	Types    string `json:"Stacktrace.frame.types"`
}

func (s StackTrace) MarshalJSON() ([]byte, error) {
	type Alias StackTrace
	return json.Marshal(struct {
		Alias
		Timestamp int64 `json:"@timestamp"`
	}{
		Alias:     Alias(s),
		Timestamp: time.Now().UnixMilli(),
	})
}

// StackFrame represents a stacktrace serializable into the stackframes index.
// DocID should be the base64-encoded FileID+Address (24 bytes).
// To simplify the unmarshalling for readers, we use arrays here, even though host agent
// doesn't send inline information yet. The symbolizer already stores arrays, which requires
// the reader to handle both formats if we don't use arrays here.
type StackFrame struct {
	EcsVersion
	DocID          string   `json:"-"`
	FileName       []string `json:"Stackframe.file.name,omitempty"`
	FunctionName   []string `json:"Stackframe.function.name,omitempty"`
	LineNumber     []int32  `json:"Stackframe.line.number,omitempty"`
	FunctionOffset []int32  `json:"Stackframe.function.offset,omitempty"`
}

func (f StackFrame) MarshalJSON() ([]byte, error) {
	type Alias StackFrame
	return json.Marshal(struct {
		Alias
		Timestamp int64 `json:"@timestamp"`
	}{
		Alias:     Alias(f),
		Timestamp: time.Now().UnixMilli(),
	})
}

// ResourceData represents the resources metadata related to a sample for the
// profiling-hosts index.
type ResourceData struct {
	EcsVersion
	HostID string `json:"host.id"`
	Data   map[string]string
}

// MarshalJSON customizes the JSON marshaling for HostResourceData.
func (h ResourceData) MarshalJSON() ([]byte, error) {
	// Create a temporary map to hold the combined data
	combinedData := make(map[string]any)

	combinedData[string(conventions.HostIDKey)] = h.HostID
	combinedData["ecs.version"] = h.V
	// The ES index profiling-hosts expects a second-precise timestamp
	combinedData["@timestamp"] = time.Now().UTC().Unix()

	// Iterate over the Data map and add the key-value pairs with lowercase keys and values
	for key, value := range h.Data {
		if value == "" {
			// Do not populate keys without value
			continue
		}
		combinedData[strings.ToLower(key)] = strings.ToLower(value)
	}

	// Marshal the combined map into JSON
	return json.Marshal(combinedData)
}

// ExeMetadata represents executable metadata serializable into the executables data stream.
// DocID should be the base64-encoded FileID.
type ExeMetadata struct {
	DocID    string `json:"-"`
	LastSeen uint32 `json:"-"`
	BuildID  string `json:"-"`
	FileName string `json:"-"`
}

func (e ExeMetadata) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"@timestamp":           e.LastSeen,
		"ecs.version":          EcsVersionString,
		"Executable.build.id":  e.BuildID,
		"Executable.file.name": e.FileName,
	})
}

func NewExeMetadata(docID string, lastSeen uint32, buildID, fileName string) ExeMetadata {
	return ExeMetadata{
		DocID:    docID,
		LastSeen: lastSeen,
		BuildID:  buildID,
		FileName: fileName,
	}
}

// UnsymbolizedExecutable represents an array of executable FileIDs written into the
// executable symbolization queue index.
type UnsymbolizedExecutable struct {
	EcsVersion
	DocID   string    `json:"-"`
	FileID  []string  `json:"Executable.file.id"`
	Created time.Time `json:"Time.created"`
	Next    time.Time `json:"Symbolization.time.next"`
	Retries int       `json:"Symbolization.retries"`
}

// UnsymbolizedLeafFrame represents an array of frame IDs written into the
// leaf frame symbolization queue index.
type UnsymbolizedLeafFrame struct {
	EcsVersion
	DocID   string    `json:"-"`
	FrameID []string  `json:"Stacktrace.frame.id"`
	Created time.Time `json:"Time.created"`
	Next    time.Time `json:"Symbolization.time.next"`
	Retries int       `json:"Symbolization.retries"`
}
