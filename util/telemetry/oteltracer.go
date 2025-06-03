package telemetry

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// default Tracer
func Tracer() trace.Tracer {
	return otel.Tracer("capg")
}
