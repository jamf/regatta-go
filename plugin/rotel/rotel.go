// Copyright JAMF Software, LLC

package rprom

import (
	"context"
	"fmt"
	"unicode/utf8"

	client "github.com/jamf/regatta-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "github.com/jamf/regatta-go/plugin/rotel"

// NewTracing returns a new Tracing that adds opentelemetry tracing to the client.
func NewTracing(opts ...Opt) *Tracing {
	t := &Tracing{}
	for _, opt := range opts {
		opt.apply(t)
	}
	if t.tracerProvider == nil {
		t.tracerProvider = otel.GetTracerProvider()
	}
	t.tracer = t.tracerProvider.Tracer(
		instrumentationName,
		trace.WithInstrumentationVersion(semVersion()),
		trace.WithSchemaURL(semconv.SchemaURL),
	)
	return t
}

type Tracing struct {
	tracerProvider trace.TracerProvider
	tracer         trace.Tracer
	keyFormatter   func([]byte) (string, error)
}

func (t *Tracing) OnKVCall(ctx context.Context, table string, op client.Op, fn client.KvDo) (client.OpResponse, error) {
	// Set up the span options.
	attrs := []attribute.KeyValue{
		semconv.DBSystemKey.String("regatta"),
		semconv.DBName(table),
	}
	spanName, attrs := t.opAttrs(attrs, op)
	opts := []trace.SpanStartOption{
		trace.WithAttributes(attrs...),
		trace.WithSpanKind(trace.SpanKindClient),
	}

	ctx, span := t.tracer.Start(ctx, spanName, opts...)
	defer span.End()

	response, err := fn(ctx, table, op)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return response, err
}

func (t *Tracing) opAttrs(attrs []attribute.KeyValue, op client.Op) (string, []attribute.KeyValue) {
	switch {
	case op.IsGet():
		attrs = append(attrs, semconv.DBOperation("get"))
		attrs = append(attrs, semconv.DBStatement(fmt.Sprintf("regatta.v1.kv/Range key: %s rangeEnd: %s", t.formatKey(op.KeyBytes()), t.formatKey(op.RangeBytes()))))
		return "regatta/get", attrs
	case op.IsPut():
		attrs = append(attrs, semconv.DBOperation("put"))
		attrs = append(attrs, semconv.DBStatement(fmt.Sprintf("regatta.v1.kv/Put key: %s", t.formatKey(op.KeyBytes()))))
		return "regatta/put", attrs
	case op.IsDelete():
		attrs = append(attrs, semconv.DBOperation("delete"))
		attrs = append(attrs, semconv.DBStatement(fmt.Sprintf("regatta.v1.kv/DeleteRange key: %s rangeEnd: %s", t.formatKey(op.KeyBytes()), t.formatKey(op.RangeBytes()))))
		return "regatta/delete", attrs
	case op.IsTxn():
		attrs = append(attrs, semconv.DBOperation("txn"))
		attrs = append(attrs, semconv.DBStatement(fmt.Sprintf("regatta.v1.kv/Txn ops")))
		return "regatta/txn", attrs
	}
	return "regatta/unknown", attrs
}

func (t *Tracing) formatKey(bytes []byte) string {
	if bytes == nil {
		return ""
	}
	var keykey string
	if t.keyFormatter != nil {
		k, err := t.keyFormatter(bytes)
		if err != nil || !utf8.ValidString(k) {
			return ""
		}
		keykey = k
	} else {
		if !utf8.Valid(bytes) {
			return ""
		}
		keykey = string(bytes)
	}
	return keykey
}
