// Schildw√§chter's Genteel Beacon
// Copyright Carsten Thiel 2025
//
// SPDX-Identifier: Apache-2.0

package o11y

import (
	"context"
	"log"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	GreaseBuildupGaugeProm prometheus.Gauge
	InkDepletionGaugeProm  prometheus.Gauge
	GreaseBuildupGaugeOtel metric.Int64ObservableGauge
	InkDepletionGaugeOtel  metric.Int64ObservableGauge
)

// InitGenteelGauges sets up metrics in both OTEL and Proemtheus
func InitGenteelGauges(appName string, commonAttribs []attribute.KeyValue, greaseBuildup *int64, inkDepletion *int64) error {
	meterProvider := otel.GetMeterProvider()
	meter := meterProvider.Meter(appName)

	// register the OTEL metrics
	GreaseBuildupGaugeOtel, _ = meter.Int64ObservableGauge(
		"genteelbeacon_greasebuildup",
		metric.WithDescription("The Genteel Beacon's current grease buildup"),
	)
	InkDepletionGaugeOtel, _ = meter.Int64ObservableGauge(
		"genteelbeacon_inkdepletion",
		metric.WithDescription("The Genteel Beacon's current ink depletion"),
	)

	promLabels := make(prometheus.Labels)
	for _, attr := range commonAttribs {
		promLabels[string(attr.Key)] = attr.Value.AsString()
	}

	// register the Prometheus metrics
	GreaseBuildupGaugeProm = promauto.NewGauge(prometheus.GaugeOpts{
		Name:        "genteelbeacon_greasebuildup_p",
		Help:        "The Genteel Beacon's current grease buidlup",
		ConstLabels: promLabels,
	})
	InkDepletionGaugeProm = promauto.NewGauge(prometheus.GaugeOpts{
		Name:        "genteelbeacon_inkdepletion_p",
		Help:        "The Genteel Beacon's current ink depletion",
		ConstLabels: promLabels,
	})

	// OTEL sending as callback on meter activity (from the channel handlers)
	var err error = nil
	_, err = meter.RegisterCallback(
		func(ctx context.Context, observer metric.Observer) error {
			// return the global value
			observer.ObserveInt64(GreaseBuildupGaugeOtel, *greaseBuildup, metric.WithAttributes(commonAttribs...))
			return nil
		}, GreaseBuildupGaugeOtel)

	if err != nil {
		log.Fatalf("Failed to register callback: %v", err)
	}
	_, err = meter.RegisterCallback(
		func(ctx context.Context, observer metric.Observer) error {
			// return the global value
			observer.ObserveInt64(InkDepletionGaugeOtel, *inkDepletion, metric.WithAttributes(commonAttribs...))
			return nil
		}, InkDepletionGaugeOtel)

	if err != nil {
		log.Fatalf("Failed to register callback: %v", err)
	}
	return err
}
