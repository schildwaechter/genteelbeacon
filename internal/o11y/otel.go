// Schildwächter's Genteel Beacon
// Copyright Carsten Thiel 2025
//
// SPDX-Identifier: Apache-2.0

package o11y

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// InitTracer initializes an OpenTelemetry tracer provider that exports traces to the specified OTLP HTTP endpoint.
func InitTracer(otlphttpEndpoint string, commonAttribs []attribute.KeyValue) (*sdktrace.TracerProvider, error) {
	ctx := context.Background()
	exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpoint(otlphttpEndpoint), otlptracehttp.WithInsecure())
	if err != nil {
		return nil, err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			commonAttribs...,
		)),
	)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return tracerProvider, nil
}

// InitMeter initializes an OpenTelemetry meter provider that exports metrics to the specified OTLP HTTP endpoint.
func InitMeter(otlphttpEndpoint string, commonAttribs []attribute.KeyValue) (*sdkmetric.MeterProvider, error) {
	ctx := context.Background()
	metricExporter, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpoint(otlphttpEndpoint), otlpmetrichttp.WithInsecure())
	if err != nil {
		return nil, err
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			commonAttribs...,
		)),
	)
	otel.SetMeterProvider(meterProvider)

	return meterProvider, nil
}

// InitOtelLogger initializes an OpenTelemetry logger provider that exports logs to the specified OTLP HTTP endpoint.
func InitOtelLogger(otlphttpEndpoint string, commonAttribs []attribute.KeyValue) (*sdklog.LoggerProvider, error) {
	ctx := context.Background()
	logExporter, err := otlploghttp.New(ctx, otlploghttp.WithEndpoint(otlphttpEndpoint), otlploghttp.WithInsecure())
	if err != nil {
		return nil, err
	}

	logProvider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			commonAttribs...,
		)),
	)
	global.SetLoggerProvider(logProvider)

	return logProvider, nil
}
