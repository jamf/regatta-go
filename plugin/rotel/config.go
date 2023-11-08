// Copyright JAMF Software, LLC

package rprom

import (
	"go.opentelemetry.io/otel/trace"
)

// Opt interface used for setting optional config properties.
type Opt interface{ apply(tracing *Tracing) }

type tracerOptFunc func(*Tracing)

func (o tracerOptFunc) apply(t *Tracing) { o(t) }

// TracerProvider takes a trace.TracerProvider and applies it to the Tracer.
// If none is specified, the global provider is used.
func TracerProvider(provider trace.TracerProvider) Opt {
	return tracerOptFunc(func(t *Tracing) { t.tracerProvider = provider })
}

// KeyFormatter formats a Record's key for use in a span's attributes,
// overriding the default of string(Record.Key).
//
// This option can be used to parse binary data and return a canonical string
// representation. If the returned string is not valid UTF-8 or if the
// formatter returns an error, the key is not attached to the span.
func KeyFormatter(fn func(key []byte) (string, error)) Opt {
	return tracerOptFunc(func(t *Tracing) { t.keyFormatter = fn })
}
