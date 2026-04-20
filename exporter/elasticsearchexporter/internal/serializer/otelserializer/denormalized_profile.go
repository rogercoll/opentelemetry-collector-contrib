// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer"

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

// ProfileDataset returns a data stream dataset name derived from the profile's
// period type and sample type (e.g. "cpu_samples"). This encodes profile-level
// metadata that would otherwise be duplicated on every sample document.
func ProfileDataset(dic pprofile.ProfilesDictionary, profile pprofile.Profile) string {
	periodType := dicString(dic, int(profile.PeriodType().TypeStrindex()))
	sampleType := dicString(dic, int(profile.SampleType().TypeStrindex()))
	if periodType == "" {
		periodType = "unknown"
	}
	if sampleType == "" {
		sampleType = "unknown"
	}
	return periodType + "_" + sampleType
}

// SerializeDenormalizedProfile serializes a profile into denormalized documents,
// one per sample, with fully resolved stack traces inline. Each document is
// self-contained and routed to a profiles-*.otel-* data stream.
func (*Serializer) SerializeDenormalizedProfile(
	dic pprofile.ProfilesDictionary,
	resource pcommon.Resource,
	resourceSchemaURL string,
	scope pcommon.InstrumentationScope,
	scopeSchemaURL string,
	profile pprofile.Profile,
	idx elasticsearch.Index,
	pushData func(*bytes.Buffer) error,
) error {
	profileTimestamp := profile.Time()
	profileID := profile.ProfileID()
	docIdx := 0

	for _, sample := range profile.Samples().All() {
		frames, err := resolveStackFrames(dic, sample)
		if err != nil {
			return fmt.Errorf("failed to resolve stack frames: %w", err)
		}

		sampleAttrs := resolveSampleAttributes(dic, sample)
		stackTraceID := computeStackTraceID(frames)

		sampleValue := int64(0)
		if sample.Values().Len() > 0 {
			sampleValue = sample.Values().At(0)
		}

		timestamps := sample.TimestampsUnixNano()
		if timestamps.Len() == 0 {
			buf := new(bytes.Buffer)
			writeDenormalizedDoc(buf, idx, resource, resourceSchemaURL, scope, scopeSchemaURL,
				profileTimestamp, profile, stackTraceID, computeSampleHash(profileID, docIdx), frames, sampleAttrs, sampleValue)
			docIdx++
			if err := pushData(buf); err != nil {
				return err
			}
			continue
		}

		for i := range timestamps.Len() {
			ts := pcommon.Timestamp(timestamps.At(i))
			val := sampleValue
			if i < sample.Values().Len() {
				val = sample.Values().At(i)
			}
			buf := new(bytes.Buffer)
			writeDenormalizedDoc(buf, idx, resource, resourceSchemaURL, scope, scopeSchemaURL,
				ts, profile, stackTraceID, computeSampleHash(profileID, docIdx), frames, sampleAttrs, val)
			docIdx++
			if err := pushData(buf); err != nil {
				return err
			}
		}
	}
	return nil
}

type resolvedFrame struct {
	functionName string
	fileName     string
	sourceLine   int32
	address      uint64
	frameType    string
	mappingFile  string
	buildID      string
}

func resolveStackFrames(dic pprofile.ProfilesDictionary, sample pprofile.Sample) ([]resolvedFrame, error) {
	stack := dic.StackTable().At(int(sample.StackIndex()))
	locations := make([]pprofile.Location, 0, stack.LocationIndices().Len())
	for _, i := range stack.LocationIndices().All() {
		locations = append(locations, dic.LocationTable().At(int(i)))
	}

	frames := make([]resolvedFrame, 0, len(locations))
	for _, location := range locations {
		if location.MappingIndex() >= int32(dic.MappingTable().Len()) {
			continue
		}

		frameTypeStr := ""
		if v, err := getStringAttr(dic, location, string(conventions.ProfileFrameTypeKey)); err == nil {
			frameTypeStr = v
		}

		var mappingFile, buildIDStr string
		if location.MappingIndex() > 0 {
			mapping := dic.MappingTable().At(int(location.MappingIndex()))
			mappingFile = dicString(dic, int(mapping.FilenameStrindex()))
			if bid, err := getStringAttr(dic, mapping, string(conventions.ProcessExecutableBuildIDHtlhashKey)); err == nil {
				buildIDStr = bid
			}
		}

		for _, line := range location.Lines().All() {
			var funcName, fileName string
			if line.FunctionIndex() < int32(dic.FunctionTable().Len()) {
				f := dic.FunctionTable().At(int(line.FunctionIndex()))
				funcName = dicString(dic, int(f.NameStrindex()))
				fileName = dicString(dic, int(f.FilenameStrindex()))
			}
			frames = append(frames, resolvedFrame{
				functionName: funcName,
				fileName:     fileName,
				sourceLine:   int32(line.Line()),
				address:      location.Address(),
				frameType:    frameTypeStr,
				mappingFile:  mappingFile,
				buildID:      buildIDStr,
			})
		}
	}

	// Frames are kept in leaf-first order (index 0 = currently executing function),
	// matching the OTel profiles spec, pprof convention, and the ES index template.
	return frames, nil
}

type attrHolder interface {
	AttributeIndices() pcommon.Int32Slice
}

func getStringAttr(dic pprofile.ProfilesDictionary, record attrHolder, key string) (string, error) {
	for _, idx := range record.AttributeIndices().All() {
		if int(idx) >= dic.AttributeTable().Len() {
			continue
		}
		attr := dic.AttributeTable().At(int(idx))
		k := dicString(dic, int(attr.KeyStrindex()))
		if k == key {
			return attr.Value().AsString(), nil
		}
	}
	return "", errors.New("attribute not found")
}

func resolveSampleAttributes(dic pprofile.ProfilesDictionary, sample pprofile.Sample) map[string]string {
	attrs := make(map[string]string)
	for _, idx := range sample.AttributeIndices().All() {
		if int(idx) >= dic.AttributeTable().Len() {
			continue
		}
		attr := dic.AttributeTable().At(int(idx))
		key := dicString(dic, int(attr.KeyStrindex()))
		val := attr.Value().AsString()
		attrs[key] = val
	}
	return attrs
}

func dicString(dic pprofile.ProfilesDictionary, index int) string {
	if index < dic.StringTable().Len() {
		return dic.StringTable().At(index)
	}
	return ""
}

func computeStackTraceID(frames []resolvedFrame) string {
	if len(frames) == 0 {
		return ""
	}
	h := fnv.New128a()
	for _, f := range frames {
		_, _ = h.Write([]byte(f.functionName))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// computeSampleHash produces a unique document-level hash used as a TSDS dimension
// to prevent version_conflict_engine_exception when multiple samples share the same
// stack_trace_id and timestamp (workaround for ES TSDS TSID collision).
func computeSampleHash(profileID pprofile.ProfileID, docIdx int) string {
	h := fnv.New128a()
	_, _ = h.Write(profileID[:])
	_, _ = h.Write([]byte{
		byte(docIdx),
		byte(docIdx >> 8),
		byte(docIdx >> 16),
		byte(docIdx >> 24),
	})
	return hex.EncodeToString(h.Sum(nil))
}

func writeDenormalizedDoc(
	buf *bytes.Buffer,
	idx elasticsearch.Index,
	resource pcommon.Resource,
	resourceSchemaURL string,
	scope pcommon.InstrumentationScope,
	scopeSchemaURL string,
	timestamp pcommon.Timestamp,
	profile pprofile.Profile,
	stackTraceID string,
	sampleHash string,
	frames []resolvedFrame,
	sampleAttrs map[string]string,
	sampleValue int64,
) {
	w := newJSONWriter(buf)
	w.startObject()
	first := true

	first = w.writeTimestampField("@timestamp", timestamp, first)
	first = w.writeDataStream(idx, first)
	first = w.writeResource(resource, resourceSchemaURL, false, first)
	first = w.writeScope(scope, scopeSchemaURL, false, first)
	first = w.writeStringFieldSkipDefault("_sample_hash", sampleHash, first)

	if len(sampleAttrs) > 0 {
		first = w.key("attributes", first)
		w.startObject()
		firstAttr := true
		for k, v := range sampleAttrs {
			firstAttr = w.key(k, firstAttr)
			w.jsonString(v)
		}
		w.endObject()
	}

	first = w.key("profile", first)
	_ = first
	w.startObject()
	pFirst := true

	profileID := profile.ProfileID()
	if !profileID.IsEmpty() {
		pFirst = w.key("id", pFirst)
		w.buf.WriteByte('"')
		b := hex.AppendEncode(w.buf.AvailableBuffer(), profileID[:])
		w.buf.Write(b)
		w.buf.WriteByte('"')
	}
	pFirst = w.writeStringFieldSkipDefault("stack_trace_id", stackTraceID, pFirst)

	pFirst = w.key("sample", pFirst)
	w.startObject()
	w.key("value", true)
	w.int64Val(sampleValue)
	w.endObject()

	if len(frames) > 0 {
		// top_frame: leaf frame (index 0) as flat keyword fields.
		// Survives downsampling as a label. Enables fast leaf-node aggregations
		// and "which function is currently on-CPU" queries without array traversal.
		top := frames[0]
		pFirst = w.key("top_frame", pFirst)
		w.startObject()
		tfFirst := true
		tfFirst = w.writeStringFieldSkipDefault("function_name", top.functionName, tfFirst)
		tfFirst = w.writeStringFieldSkipDefault("frame_type", top.frameType, tfFirst)
		if top.mappingFile != "" {
			_ = w.key("mapping", tfFirst)
			w.startObject()
			_ = w.writeStringFieldSkipDefault("filename", top.mappingFile, true)
			w.endObject()
		}
		w.endObject()

		pFirst = w.key("stack_trace", pFirst)
		w.startArray()
		for i, frame := range frames {
			if i > 0 {
				w.buf.WriteByte(',')
			}
			w.startObject()
			fFirst := true
			fFirst = w.writeStringFieldSkipDefault("function_name", frame.functionName, fFirst)
			fFirst = w.writeStringFieldSkipDefault("file_name", frame.fileName, fFirst)
			if frame.sourceLine > 0 {
				fFirst = w.key("source_line", fFirst)
				w.int64Val(int64(frame.sourceLine))
			}
			if frame.address > 0 {
				fFirst = w.key("address", fFirst)
				w.jsonString(fmt.Sprintf("%d", frame.address))
			}
			fFirst = w.writeStringFieldSkipDefault("frame_type", frame.frameType, fFirst)
			if frame.mappingFile != "" || frame.buildID != "" {
				_ = w.key("mapping", fFirst)
				w.startObject()
				mFirst := true
				mFirst = w.writeStringFieldSkipDefault("filename", frame.mappingFile, mFirst)
				_ = w.writeStringFieldSkipDefault("build_id", frame.buildID, mFirst)
				w.endObject()
			}
			w.endObject()
		}
		w.endArray()

		// all_frames: concatenated function names for LLM-friendly text search
		_ = w.key("all_frames", pFirst)
		var allNames []string
		for _, frame := range frames {
			if frame.functionName != "" {
				allNames = append(allNames, frame.functionName)
			}
		}
		w.jsonString(strings.Join(allNames, " -> "))
	}
	w.endObject() // profile
	w.endObject() // root
}
