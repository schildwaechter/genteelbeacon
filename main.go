// Schildwächter's Genteel Beacon
// Copyright Carsten Thiel 2025
//
// SPDX-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"time"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/contrib/otelfiber/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	slogfiber "github.com/samber/slog-fiber"
	slogmulti "github.com/samber/slog-multi"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/contrib/bridges/otelslog"

	"genteelbeacon/gearsmith"
)

var (
	buildEpoch            string = "0"
	startTime             time.Time
	greaseFactor          float64 = 1.0
	inkNeed               float64 = 0.0
	greaseFactorGaugeProm prometheus.Gauge
	inkNeedGaugeProm      prometheus.Gauge
	greaseFactorGaugeOtel metric.Float64ObservableGauge
	inkNeedGaugeOtel      metric.Float64ObservableGauge
	totalGearAnswers      int64 = 0
	totalInkAnswers       int64 = 0
	tracer                trace.Tracer
	logger                *slog.Logger
)

// Get environment variable with a default
func getEnv(name string, defaultValue string) string {
	value, exists := os.LookupEnv(name)
	if exists {
		return value
	}
	return defaultValue
}

func initTracer(otlphttpEndpoint string, serviceName string) (*sdktrace.TracerProvider, error) {
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
			semconv.ServiceNameKey.String(serviceName))),
	)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return tracerProvider, nil
}

func initMeter(otlphttpEndpoint string, serviceName string) (*sdkmetric.MeterProvider, error) {
	ctx := context.Background()
	metricExporter, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpoint(otlphttpEndpoint), otlpmetrichttp.WithInsecure())
	if err != nil {
		return nil, err
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)),
	)
	otel.SetMeterProvider(meterProvider)

	return meterProvider, nil
}

func initLogger(otlphttpEndpoint string, serviceName string) (*sdklog.LoggerProvider, error) {
	ctx := context.Background()
	logExporter, err := otlploghttp.New(ctx, otlploghttp.WithEndpoint(otlphttpEndpoint), otlploghttp.WithInsecure())
	if err != nil {
		return nil, err
	}

	logProvider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)),
	)
	global.SetLoggerProvider(logProvider)

	return logProvider, nil
}

func loggerTraceAttr(ctx context.Context, span trace.Span) slog.Attr {
	var trace_attr slog.Attr
	if trace.SpanFromContext(ctx).SpanContext().HasTraceID() {
		trace_attr = slog.String("trace_id", span.SpanContext().TraceID().String())
	}
	return trace_attr
}
func loggerSpanAttr(ctx context.Context, span trace.Span) slog.Attr {
	var span_attr slog.Attr
	if trace.SpanFromContext(ctx).SpanContext().HasSpanID() {
		span_attr = slog.String("span_id", span.SpanContext().SpanID().String())
	}
	return span_attr
}

func greaseGrate(ctx context.Context, tracer trace.Tracer) error {
	_, span := tracer.Start(ctx, "Grease Grate")
	defer span.End()

	// Whether to trip (between 0 and 1)
	tripValue := rand.Float64()
	// The threshold to trip the grease grate:
	// not below 0.9, increasing probablility from 0.9-1 and always above
	tripThreshold := (greaseFactor - 0.9) * 10

	slog.Info(fmt.Sprintf("greaseFactor %f - tripThreshold %f - tripValue %f", greaseFactor, tripThreshold, tripValue))

	if tripValue < tripThreshold {
		err := errors.New("Grease Grate clogged 💀")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		logger.ErrorContext(ctx, err.Error(), loggerTraceAttr(ctx, span), loggerSpanAttr(ctx, span))

		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	} else {
		time.Sleep(1 * time.Millisecond) // artificial span increase
	}

	return nil
}

func scribeStudy(ctx context.Context, tracer trace.Tracer, appName string, timeString string, useClock bool, requestId string) (string, error) {
	ctx, span := tracer.Start(ctx, "Scribe Study")
	defer span.End()

	logger.DebugContext(ctx, "Scribe at work 🖊️")

	nodeName, err := os.Hostname()
	if err != nil {
		nodeName = "unknown host"
	}

	span.AddEvent("Preparing message")
	scribeErrorChance := rand.Float64()
	scribeSignature := "Build " + buildEpoch + ", »" + appName + "« running on " + nodeName + " 🙋 " + requestId
	var scribeMessage string
	if useClock {
		scribeMessage = "🕰️ The time is " + timeString + "\n"
	} else {
		scribeMessage = "📅 Today is " + timeString + " – that's all we have!\n"
	}

	if scribeErrorChance < 0.01 { // very rare super long delay
		span.AddEvent("Pan search")
		logger.WarnContext(ctx, "Scribe dropped the pen 🔍!!", loggerTraceAttr(ctx, span), loggerSpanAttr(ctx, span))
		time.Sleep(3 * time.Second) // uppss...
	} else if scribeErrorChance > 0.99 { // somestimes it can't wait
		span.AddEvent("Break time")
		err := errors.New("Scribe seems to be having a break 🫖")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		logger.ErrorContext(ctx, err.Error(), loggerTraceAttr(ctx, span), loggerSpanAttr(ctx, span))

		return "🕰️ The time is not available at this moment!!" + scribeSignature, fiber.NewError(fiber.StatusTeapot, err.Error())
	} else if scribeErrorChance > 0.96 { // oh dear (if we haven't tripped before)
		span.AddEvent("Urgent need")
		err := errors.New("Scribe seems to be indisposed 💩")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		logger.ErrorContext(ctx, err.Error(), loggerTraceAttr(ctx, span), loggerSpanAttr(ctx, span))

		return "🕰️ The time is not available at this moment!!" + scribeSignature, fiber.NewError(fiber.StatusServiceUnavailable, err.Error())
	} else {
		time.Sleep(time.Duration(rand.IntN(70)+1) * time.Millisecond) // normal artificial span increase
		span.AddEvent("Message ready")
	}

	return scribeMessage + scribeSignature, nil
}

func courteousCourier(ctx context.Context, tracer trace.Tracer, client *http.Client, clock string) (error, string) {
	_, span := tracer.Start(ctx, "CourteousCourier")
	defer span.End()

	logger.DebugContext(ctx, "Courier checking "+clock+" 🐦", loggerTraceAttr(ctx, span), loggerSpanAttr(ctx, span))

	req, err := http.NewRequestWithContext(ctx, "GET", clock+"/timestamp", nil)

	// Inject TraceParent to Context
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := client.Do(req)
	if err != nil {
		span.RecordError(err)
		logger.ErrorContext(ctx, "Error checking clock!", loggerTraceAttr(ctx, span), loggerSpanAttr(ctx, span))
		return err, "Error checking clock!"
	}
	defer resp.Body.Close()

	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		span.RecordError(err)
		logger.ErrorContext(ctx, err.Error())
		return nil, ""
	}

	return nil, string(responseData)
}

func initGenteelGauges(appName string) error {
	meterProvider := otel.GetMeterProvider()
	meter := meterProvider.Meter(appName)

	// register the OTEL metrics
	greaseFactorGaugeOtel, _ = meter.Float64ObservableGauge(
		"genteelbeacon_greasefactor",
		metric.WithDescription("The Genteel Beacon's current grease factor"),
	)
	inkNeedGaugeOtel, _ = meter.Float64ObservableGauge(
		"genteelbeacon_inkneed",
		metric.WithDescription("The Genteel Beacon's current ink need"),
	)

	// register the Prometheus metrics
	greaseFactorGaugeProm = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "genteelbeacon_greasefactor_p",
		Help: "The Genteel Beacon's current grease factor",
	})
	inkNeedGaugeProm = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "genteelbeacon_inkneed_p",
		Help: "The Genteel Beacon's current ink need",
	})

	// start a go routine to update the values
	go func() {
		for {
			greaseFactor = float64(totalGearAnswers) / (2 * (time.Since(startTime).Seconds() + 10))
			greaseFactorGaugeProm.Set(greaseFactor)
			inkNeed = float64(totalInkAnswers) / (time.Since(startTime).Seconds() + 10)
			inkNeedGaugeProm.Set(inkNeed)
			time.Sleep(1 * time.Second)
		}
	}()

	// OTEL sending
	var err error = nil
	_, err = meter.RegisterCallback(
		func(ctx context.Context, observer metric.Observer) error {
			nodeName, err := os.Hostname()
			if err != nil {
				nodeName = "unknown host"
			}
			hostName := []attribute.KeyValue{attribute.String("hostname", nodeName)}

			// return the global value
			observer.ObserveFloat64(greaseFactorGaugeOtel, greaseFactor, metric.WithAttributes(hostName...))

			return nil
		}, greaseFactorGaugeOtel)

	if err != nil {
		log.Fatalf("Failed to register callback: %v", err)
	}
	_, err = meter.RegisterCallback(
		func(ctx context.Context, observer metric.Observer) error {
			nodeName, err := os.Hostname()
			if err != nil {
				nodeName = "unknown host"
			}
			hostName := []attribute.KeyValue{attribute.String("hostname", nodeName)}

			// return the global value
			observer.ObserveFloat64(inkNeedGaugeOtel, inkNeed, metric.WithAttributes(hostName...))

			return nil
		}, inkNeedGaugeOtel)

	if err != nil {
		log.Fatalf("Failed to register callback: %v", err)
	}
	return err
}

func main() {
	// our name and role
	appName := getEnv("APP_NAME", "Genteel Beacon")
	genteelRole := getEnv("GENTEEL_ROLE", "Default")

	startTime = time.Now()

	app := fiber.New()
	app.Use(requestid.New())

	app.Use(healthcheck.New(healthcheck.Config{
		LivenessProbe: func(c *fiber.Ctx) bool {
			return true
		},
		LivenessEndpoint: "/livez",
		ReadinessProbe: func(c *fiber.Ctx) bool {
			if greaseFactor < 0.9 {
				return true
			} else {
				return false
			}
		},
		ReadinessEndpoint: "/readyz",
	}))

	initGenteelGauges(appName)
	prometheus := fiberprometheus.NewWithDefaultRegistry(appName)
	prometheus.RegisterAt(app, "/metrics")

	app.Use(prometheus.Middleware)

	app.Use(otelfiber.Middleware())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Genteel Beacon 🚨")
	})

	otlphttpEndpoint, ok := os.LookupEnv("OTLPHTTP_ENDPOINT")
	if ok {
		tp, err := initTracer(otlphttpEndpoint, appName)
		if err != nil {
			slog.Error("Can't send traces")
		}
		defer func() {
			_ = tp.Shutdown(context.Background())
		}()
		mp, err := initMeter(otlphttpEndpoint, appName)
		if err != nil {
			log.Fatal("Can't send metrics")
		}
		defer func() {
			_ = mp.Shutdown(context.Background())
		}()
		lp, err := initLogger(otlphttpEndpoint, appName)
		if err != nil {
			log.Fatal("Can't send logs")
		}
		defer func() {
			_ = lp.Shutdown(context.Background())
		}()
		slog.Info("Sending OTEL data to " + otlphttpEndpoint)
	} else {
		slog.Info("Not sending OTEL data")
	}
	_, jsonLogging := os.LookupEnv("JSONLOGGING")
	if jsonLogging {
		logger = slog.New(
			slogmulti.Fanout(
				slog.Default().Handler(),
				slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}),
			),
		)
	} else {
		logger = slog.New(
			slogmulti.Fanout(
				otelslog.NewLogger(appName).Handler(),
				slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
			),
		)
	}
	loggerConfig := slogfiber.Config{
		WithSpanID:    true,
		WithTraceID:   true,
		WithRequestID: true,
	}
	app.Use(slogfiber.NewWithConfig(logger, loggerConfig))
	app.Use(recover.New())

	client := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	tracer = otel.Tracer(appName)

	if genteelRole == "gearsmith" {
		gearsmith.RunGearsmith()
	} else {

		app.Get("/timestamp", func(c *fiber.Ctx) error {
			ctx, span := tracer.Start(c.UserContext(), "Timestamp Endpoint")
			span.SetAttributes(attribute.String("RequestID", slogfiber.GetRequestIDFromContext(c.Context())))
			defer span.End()

			totalGearAnswers += 1
			greaseErr := greaseGrate(ctx, tracer)

			if greaseErr != nil {
				return greaseErr
			}

			theTime := time.Now().Format("2006-01-02 15:04:05")
			return c.SendString(theTime)
		})

		// Define the route for the main path '/telegram'
		app.Get("/telegram", func(c *fiber.Ctx) error {
			ctx, span := tracer.Start(c.UserContext(), "Telegram Endpoint")
			span.SetAttributes(attribute.String("RequestID", slogfiber.GetRequestIDFromContext(c.Context())))
			defer span.End()

			totalInkAnswers += 1

			var clockString string
			var clockResponseError error = nil
			clock, useClock := os.LookupEnv("GENTEEL_CLOCK")

			if useClock {
				clockResponseError, clockString = courteousCourier(ctx, tracer, client, clock)
			} else {
				logger.DebugContext(ctx, "No clock available")
				clockString = time.Now().Format("2006-01-02")
			}
			if clockResponseError != nil {
				return clockResponseError
			}

			scribeStudyMessage, scribeErr := scribeStudy(ctx, tracer, appName, clockString, useClock, slogfiber.GetRequestIDFromContext(c.Context()))

			if scribeErr != nil {
				return scribeErr
			}

			return c.SendString(scribeStudyMessage + " 🏁\n")
		})

		// Start the server on the specified port and address
		appPort := getEnv("APP_PORT", "1333")
		appAddr := getEnv("APP_ADDR", "0.0.0.0")
		log.Fatal(app.Listen(appAddr + ":" + appPort))
	}
}
