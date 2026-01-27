// Schildwächter's Genteel Beacon
// Copyright Carsten Thiel 2025-2026
//
// SPDX-Identifier: Apache-2.0

package services

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/schildwaechter/genteelbeacon/internal/config"
	"github.com/schildwaechter/genteelbeacon/internal/o11y"
	"github.com/schildwaechter/genteelbeacon/internal/types"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// NimbleCourier checks the remote clock
func NimbleCourier(ctx context.Context, clock string) (types.ClockReading, error) {
	_, span := otel.Tracer(config.AppName).Start(ctx, "NimbleCourier")
	defer span.End()

	// we need to make calls out
	client := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	o11y.Logger.DebugContext(ctx, "Courier checking "+clock+" üê¶", o11y.LoggerTraceAttr(ctx, span), o11y.LoggerSpanAttr(ctx, span))

	req, err := http.NewRequestWithContext(ctx, "GET", clock+"/timestamp", nil)

	// Inject TraceParent to Context
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
	var clockResponseData types.ClockReading

	resp, err := client.Do(req)
	if err != nil {
		span.RecordError(err)
		o11y.Logger.ErrorContext(ctx, "Error checking clock!", o11y.LoggerTraceAttr(ctx, span), o11y.LoggerSpanAttr(ctx, span))
		clockResponseData = types.ClockReading{
			TimeReading: "Error checking clock!",
			ClockName:   "unknown",
		}

		return clockResponseData, err
	}
	defer resp.Body.Close()

	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		span.RecordError(err)
		o11y.Logger.ErrorContext(ctx, err.Error())
		clockResponseData = types.ClockReading{
			TimeReading: err.Error(),
			ClockName:   "unknown",
		}
		return clockResponseData, nil
	}
	json.Unmarshal(responseData, &clockResponseData)

	return clockResponseData, nil
}
