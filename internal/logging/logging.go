package logging

import (
	"context"

	liblogger "github.com/Kaese72/huemie-lib/logging"
	"go.elastic.co/apm/v2"
)

func extractApmDict(ctx context.Context) map[string]interface{} {
	// Completely stolen from documentation, https://www.elastic.co/guide/en/apm/agent/go/current/log-correlation-ids.html
	// Some slight modifications to create correct types
	labels := map[string]interface{}{}
	tx := apm.TransactionFromContext(ctx)
	if tx != nil {
		traceContext := tx.TraceContext()
		labels["trace.id"] = traceContext.Trace.String()
		labels["transaction.id"] = traceContext.Span.String()
		if span := apm.SpanFromContext(ctx); span != nil {
			labels["span.id"] = span.TraceContext().Span.String()
		}
	}
	return labels
}

func Info(msg string, ctx context.Context, data ...map[string]interface{}) {
	liblogger.Info(msg, append(data, extractApmDict(ctx))...)
}

func Error(msg string, ctx context.Context, data ...map[string]interface{}) {
	liblogger.Error(msg, append(data, extractApmDict(ctx))...)
}

func Fatal(msg string, ctx context.Context, data ...map[string]interface{}) {
	liblogger.Fatal(msg, append(data, extractApmDict(ctx))...)
}
