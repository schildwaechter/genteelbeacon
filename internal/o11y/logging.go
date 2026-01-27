// Schildwächter's Genteel Beacon
// Copyright Carsten Thiel 2025-2026
//
// SPDX-Identifier: Apache-2.0

package o11y

import (
	"context"
	"log/slog"
	"os"

	slogmulti "github.com/samber/slog-multi"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/trace"
)

var Logger *slog.Logger

func CreateLogger(appName string, jsonLogging bool) {
	if jsonLogging {
		Logger = slog.New(
			slogmulti.Fanout(
				otelslog.NewLogger(appName).Handler(),
				slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}),
			),
		)
	} else {
		Logger = slog.New(
			slogmulti.Fanout(
				otelslog.NewLogger(appName).Handler(),
				slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
			),
		)
	}
}

func LoggerTraceAttr(ctx context.Context, span trace.Span) slog.Attr {
	var traceAttr slog.Attr
	if trace.SpanFromContext(ctx).SpanContext().HasTraceID() {
		traceAttr = slog.String("trace_id", span.SpanContext().TraceID().String())
	}
	return traceAttr
}

func LoggerSpanAttr(ctx context.Context, span trace.Span) slog.Attr {
	var spanAttr slog.Attr
	if trace.SpanFromContext(ctx).SpanContext().HasSpanID() {
		spanAttr = slog.String("span_id", span.SpanContext().SpanID().String())
	}
	return spanAttr
}
