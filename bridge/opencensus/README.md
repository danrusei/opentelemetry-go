# OpenCensus Bridge

The OpenCensus Bridge helps facilitate the migration of an application from OpenCensus to OpenTelemetry.

## Tracing

### The Problem: Mixing OpenCensus and OpenTelemetry libraries

In a perfect world, one would simply migrate their entire go application --including custom instrumentation, libraries, and exporters-- from OpenCensus to OpenTelemetry all at once.  In the real world, dependency constraints, third-party ownership of libraries, or other reasons may require mixing OpenCensus and OpenTelemetry libraries in a single application.

However, if you create the following spans in a go application:

```go
ctx, ocSpan := opencensus.StartSpan(context.Background(), "OuterSpan")
defer ocSpan.End()
ctx, otSpan := opentelemetryTracer.Start(ctx, "MiddleSpan")
defer otSpan.End()
ctx, ocSpan := opencensus.StartSpan(ctx, "InnerSpan")
defer ocSpan.End()
```

OpenCensus reports (to OpenCensus exporters):

```
[--------OuterSpan------------]
    [----InnerSpan------]
```

OpenTelemetry reports (to OpenTelemetry exporters):

```
   [-----MiddleSpan--------]
```

Instead, I would prefer (to a single set of exporters):

```
[--------OuterSpan------------]
   [-----MiddleSpan--------]
    [----InnerSpan------]
```

### The bridge solution

The bridge implements the OpenCensus trace API using OpenTelemetry.  This would cause, for example, a span recorded with OpenCensus' `StartSpan()` method to be equivalent to recording a span using OpenTelemetry's `tracer.Start()` method.  Funneling all tracing API calls to OpenTelemetry APIs results in the desired unified span hierarchy.

### User Journey

Starting from an application using entirely OpenCensus APIs:

1. Instantiate OpenTelemetry SDK and Exporters
2. Override OpenCensus' DefaultTracer with the bridge
3. Migrate libraries individually from OpenCensus to OpenTelemetry
4. Remove OpenCensus exporters and configuration

To override OpenCensus' DefaultTracer with the bridge:
```go
import (
	octrace "go.opencensus.io/trace"
	"go.opentelemetry.io/otel/bridge/opencensus"
	"go.opentelemetry.io/otel"
)

tracer := otel.GetTracerProvider().Tracer("bridge")
octrace.DefaultTracer = opencensus.NewTracer(tracer)
```

Be sure to set the `Tracer` name to your instrumentation package name instead of `"bridge"`.

#### Incompatibilities

OpenCensus and OpenTelemetry APIs are not entirely compatible.  If the bridge finds any incompatibilities, it will log them.  Incompatibilities include:

* Custom OpenCensus Samplers specified during StartSpan are ignored.
* Links cannot be added to OpenCensus spans.
* OpenTelemetry Debug or Deferred trace flags are dropped after an OpenCensus span is created.

## Metrics

### The problem: mixing libraries without mixing pipelines

The problem for monitoring is simpler than the problem for tracing, since there
are no context propagation issues to deal with. However, it still is difficult
for users to migrate an entire applications' monitoring at once. It
should be possible to send metrics generated by OpenCensus libraries to an 
OpenTelemetry pipeline so that migrating a metric does not require maintaining
separate export pipelines for OpenCensus and OpenTelemetry.

### The Exporter "wrapper" solution

The solution we use here is to allow wrapping an OpenTelemetry exporter such
that it implements the OpenCensus exporter interfaces. This allows a single
exporter to be used for metrics from *both* OpenCensus and OpenTelemetry.

### User Journey

Starting from an application using entirely OpenCensus APIs:

1. Instantiate OpenTelemetry SDK and Exporters.
2. Replace OpenCensus exporters with a wrapped OpenTelemetry exporter from step 1.
3. Migrate libraries individually from OpenCensus to OpenTelemetry
4. Remove OpenCensus Exporters and configuration.

For example, to swap out the OpenCensus logging exporter for the OpenTelemetry stdout exporter:
```go
import (
	"go.opencensus.io/metric/metricexport"
	"go.opentelemetry.io/otel/bridge/opencensus"
	"go.opentelemetry.io/otel/exporters/stdout" 
	"go.opentelemetry.io/otel"
)
// With OpenCensus, you could have previously configured the logging exporter like this:
//       import logexporter "go.opencensus.io/examples/exporter"
//       exporter, _ := logexporter.NewLogExporter(logexporter.Options{})
// Instead, we can create an equivalent using the OpenTelemetry stdout exporter:
openTelemetryExporter, _ := stdout.NewExporter(stdout.WithPrettyPrint())
exporter := opencensus.NewMetricExporter(openTelemetryExporter)

// Use the wrapped OpenTelemetry exporter like you normally would with OpenCensus
intervalReader, _ := metricexport.NewIntervalReader(&metricexport.Reader{}, exporter)
intervalReader.Start()
```
