// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stdout_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/stdout"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestExporter_ExportSpan(t *testing.T) {
	// write to buffer for testing
	var b bytes.Buffer
	ex, err := stdout.NewExporter(stdout.WithWriter(&b))
	if err != nil {
		t.Errorf("Error constructing stdout exporter %s", err)
	}

	// setup test span
	now := time.Now()
	traceID, _ := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	spanID, _ := trace.SpanIDFromHex("0102030405060708")
	traceState, _ := trace.TraceStateFromKeyValues(attribute.String("key", "val"))
	keyValue := "value"
	doubleValue := 123.456
	resource := resource.NewWithAttributes(attribute.String("rk1", "rv11"))

	testSpan := &tracesdk.SpanSnapshot{
		SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    traceID,
			SpanID:     spanID,
			TraceState: traceState,
		}),
		Name:      "/foo",
		StartTime: now,
		EndTime:   now,
		Attributes: []attribute.KeyValue{
			attribute.String("key", keyValue),
			attribute.Float64("double", doubleValue),
		},
		MessageEvents: []tracesdk.Event{
			{Name: "foo", Attributes: []attribute.KeyValue{attribute.String("key", keyValue)}, Time: now},
			{Name: "bar", Attributes: []attribute.KeyValue{attribute.Float64("double", doubleValue)}, Time: now},
		},
		SpanKind:      trace.SpanKindInternal,
		StatusCode:    codes.Error,
		StatusMessage: "interesting",
		Resource:      resource,
	}
	if err := ex.ExportSpans(context.Background(), []*tracesdk.SpanSnapshot{testSpan}); err != nil {
		t.Fatal(err)
	}

	expectedSerializedNow, _ := json.Marshal(now)

	got := b.String()
	expectedOutput := `[{"SpanContext":{` +
		`"TraceID":"0102030405060708090a0b0c0d0e0f10",` +
		`"SpanID":"0102030405060708","TraceFlags":"00",` +
		`"TraceState":[` +
		`{` +
		`"Key":"key",` +
		`"Value":{"Type":"STRING","Value":"val"}` +
		`}],"Remote":false},` +
		`"Parent":{` +
		`"TraceID":"00000000000000000000000000000000",` +
		`"SpanID":"0000000000000000",` +
		`"TraceFlags":"00",` +
		`"TraceState":null,` +
		`"Remote":false` +
		`},` +
		`"SpanKind":1,` +
		`"Name":"/foo",` +
		`"StartTime":` + string(expectedSerializedNow) + "," +
		`"EndTime":` + string(expectedSerializedNow) + "," +
		`"Attributes":[` +
		`{` +
		`"Key":"key",` +
		`"Value":{"Type":"STRING","Value":"value"}` +
		`},` +
		`{` +
		`"Key":"double",` +
		`"Value":{"Type":"FLOAT64","Value":123.456}` +
		`}],` +
		`"MessageEvents":[` +
		`{` +
		`"Name":"foo",` +
		`"Attributes":[` +
		`{` +
		`"Key":"key",` +
		`"Value":{"Type":"STRING","Value":"value"}` +
		`}` +
		`],` +
		`"DroppedAttributeCount":0,` +
		`"Time":` + string(expectedSerializedNow) +
		`},` +
		`{` +
		`"Name":"bar",` +
		`"Attributes":[` +
		`{` +
		`"Key":"double",` +
		`"Value":{"Type":"FLOAT64","Value":123.456}` +
		`}` +
		`],` +
		`"DroppedAttributeCount":0,` +
		`"Time":` + string(expectedSerializedNow) +
		`}` +
		`],` +
		`"Links":null,` +
		`"StatusCode":"Error",` +
		`"StatusMessage":"interesting",` +
		`"DroppedAttributeCount":0,` +
		`"DroppedMessageEventCount":0,` +
		`"DroppedLinkCount":0,` +
		`"ChildSpanCount":0,` +
		`"Resource":[` +
		`{` +
		`"Key":"rk1",` +
		`"Value":{"Type":"STRING","Value":"rv11"}` +
		`}],` +
		`"InstrumentationLibrary":{` +
		`"Name":"",` +
		`"Version":""` +
		`}}]` + "\n"

	if got != expectedOutput {
		t.Errorf("Want: %v but got: %v", expectedOutput, got)
	}
}

func TestExporterShutdownHonorsTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	e, err := stdout.NewExporter()
	if err != nil {
		t.Fatalf("failed to create exporter: %v", err)
	}

	innerCtx, innerCancel := context.WithTimeout(ctx, time.Nanosecond)
	defer innerCancel()
	<-innerCtx.Done()
	if err := e.Shutdown(innerCtx); err == nil {
		t.Error("expected context DeadlineExceeded error, got nil")
	} else if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context DeadlineExceeded error, got %v", err)
	}
}

func TestExporterShutdownHonorsCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	e, err := stdout.NewExporter()
	if err != nil {
		t.Fatalf("failed to create exporter: %v", err)
	}

	innerCtx, innerCancel := context.WithCancel(ctx)
	innerCancel()
	if err := e.Shutdown(innerCtx); err == nil {
		t.Error("expected context canceled error, got nil")
	} else if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context canceled error, got %v", err)
	}
}

func TestExporterShutdownNoError(t *testing.T) {
	e, err := stdout.NewExporter()
	if err != nil {
		t.Fatalf("failed to create exporter: %v", err)
	}

	if err := e.Shutdown(context.Background()); err != nil {
		t.Errorf("shutdown errored: expected nil, got %v", err)
	}
}
