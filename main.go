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
	"strings"
	"time"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/contrib/otelfiber/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/google/uuid"
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
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/contrib/bridges/otelslog"
)

var (
	buildEpoch    string = "0"
	greaseBuildup int64  = 0
	inkDepletion  int64  = 0
	greaseChan    chan int64
	inkChan       chan int64

	greaseBuildupGaugeProm prometheus.Gauge
	inkDepletionGaugeProm  prometheus.Gauge
	greaseBuildupGaugeOtel metric.Int64ObservableGauge
	inkDepletionGaugeOtel  metric.Int64ObservableGauge

	totalGearAnswers int64 = 0
	totalInkAnswers  int64 = 0
	tracer           trace.Tracer
	logger           *slog.Logger
)

// Get environment variable with a default
func getEnv(name string, defaultValue string) string {
	value, exists := os.LookupEnv(name)
	if exists {
		return value
	}
	return defaultValue
}

func initTracer(otlphttpEndpoint string, commonAttribs []attribute.KeyValue) (*sdktrace.TracerProvider, error) {
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

func initMeter(otlphttpEndpoint string, commonAttribs []attribute.KeyValue) (*sdkmetric.MeterProvider, error) {
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

func initLogger(otlphttpEndpoint string, commonAttribs []attribute.KeyValue) (*sdklog.LoggerProvider, error) {
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
	childCtx, span := tracer.Start(ctx, "GreaseGrate")
	defer span.End()

	// Whether to trip (between 0 and 1)
	tripValue := rand.Float64()
	// The threshold to trip the grease grate:
	// not below 0.9, increasing probablility from 0.9-1 and always above
	tripThreshold := float64(greaseBuildup-90) / 10

	logger.DebugContext(childCtx, fmt.Sprintf("greaseBuildup %d - tripThreshold %f - tripValue %f", greaseBuildup, tripThreshold, tripValue))

	if tripValue < tripThreshold {
		err := errors.New("Grease Grate clogged 💀")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		logger.ErrorContext(childCtx, err.Error(), loggerTraceAttr(ctx, span), loggerSpanAttr(childCtx, span))

		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	} else {
		time.Sleep(3 * time.Millisecond) // artificial span increase
	}

	return nil
}

func inkWell(ctx context.Context, tracer trace.Tracer) error {
	childCtx, span := tracer.Start(ctx, "InkWell")
	defer span.End()

	// Whether to trip (between 0 and 1)
	tripValue := rand.Float64()
	// The threshold to trip the grease grate:
	// not below 0.9, increasing probablility from 0.9-1 and always above
	tripThreshold := float64(inkDepletion-90) / 10

	logger.DebugContext(childCtx, fmt.Sprintf("inkDepletion %d - tripThreshold %f - tripValue %f", inkDepletion, tripThreshold, tripValue))

	if tripValue < tripThreshold {
		err := errors.New("Ink Well running dry 🐙")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		logger.ErrorContext(childCtx, err.Error(), loggerTraceAttr(ctx, span), loggerSpanAttr(childCtx, span))

		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	} else {
		time.Sleep(3 * time.Millisecond) // artificial span increase
	}

	return nil
}

func scribeStudy(ctx context.Context, tracer trace.Tracer, appName string, timeString string, useClock bool, requestId string) (string, error) {
	ctx, span := tracer.Start(ctx, "ScribeStudy")
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

func initGenteelGauges(appName string, commonAttribs []attribute.KeyValue) error {
	meterProvider := otel.GetMeterProvider()
	meter := meterProvider.Meter(appName)

	// greaseBuildupGaugeProm prometheus.Gauge
	// inkDepletionGaugeProm  prometheus.Gauge
	// greaseBuildupGaugeOtel metric.Int64Gauge
	// inkDepletionGaugeOtel  metric.Int64Gauge

	// register the OTEL metrics
	greaseBuildupGaugeOtel, _ = meter.Int64ObservableGauge(
		"genteelbeacon_greasebuildup",
		metric.WithDescription("The Genteel Beacon's current grease buildup"),
	)
	inkDepletionGaugeOtel, _ = meter.Int64ObservableGauge(
		"genteelbeacon_inkdepletion",
		metric.WithDescription("The Genteel Beacon's current ink depletion"),
	)

	// register the Prometheus metrics
	greaseBuildupGaugeProm = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "genteelbeacon_greasebuildup_p",
		Help: "The Genteel Beacon's current grease buidlup",
	})
	inkDepletionGaugeProm = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "genteelbeacon_inkdepletion_p",
		Help: "The Genteel Beacon's current ink depletion",
	})

	// OTEL sending
	var err error = nil
	_, err = meter.RegisterCallback(
		func(ctx context.Context, observer metric.Observer) error {
			// return the global value
			observer.ObserveInt64(greaseBuildupGaugeOtel, greaseBuildup, metric.WithAttributes(commonAttribs...))
			return nil
		}, greaseBuildupGaugeOtel)

	if err != nil {
		log.Fatalf("Failed to register callback: %v", err)
	}
	_, err = meter.RegisterCallback(
		func(ctx context.Context, observer metric.Observer) error {
			// return the global value
			observer.ObserveInt64(inkDepletionGaugeOtel, inkDepletion, metric.WithAttributes(commonAttribs...))
			return nil
		}, inkDepletionGaugeOtel)

	if err != nil {
		log.Fatalf("Failed to register callback: %v", err)
	}
	return err
}

func main() {
	// our name and role
	appName := getEnv("GENTEEL_NAME", "Genteel Beacon")
	genteelRole := getEnv("GENTEEL_ROLE", "Default")
	nodeName, err := os.Hostname()
	if err != nil {
		nodeName = "unknown_host"
	}
	greaseChan = make(chan int64)
	inkChan = make(chan int64)

	go func() {
		for {
			greaseChange := <-greaseChan
			if greaseChange == -1 && greaseBuildup > 0 {
				greaseBuildup--
				greaseBuildupGaugeProm.Dec()
			} else if greaseChange == 1 && rand.IntN(100) < 50 {
				greaseBuildup++
				greaseBuildupGaugeProm.Inc()
			}
		}
	}()
	go func() {
		for {
			inkChange := <-inkChan
			if inkChange == -1 && inkDepletion > 0 {
				inkDepletion--
				inkDepletionGaugeProm.Dec()
			} else if inkChange == 1 {
				inkDepletion++
				inkDepletionGaugeProm.Inc()
			}
		}
	}()

	go func() {
		for {
			greaseChan <- -1
			inkChan <- -1
			time.Sleep(1 * time.Second)
		}
	}()

	app := fiber.New()
	app.Use(requestid.New())

	app.Use(healthcheck.New(healthcheck.Config{
		LivenessProbe: func(c *fiber.Ctx) bool {
			return true
		},
		LivenessEndpoint: "/livez",
		ReadinessProbe: func(c *fiber.Ctx) bool {
			if greaseBuildup < 95 && inkDepletion < 95 {
				return true
			} else {
				return false
			}
		},
		ReadinessEndpoint: "/readyz",
	}))

	commonAttribs := []attribute.KeyValue{
		semconv.ServiceNameKey.String(strings.ToLower(strings.ReplaceAll(appName, " ", ""))),
		semconv.ServiceInstanceIDKey.String(uuid.New().String()),
		attribute.String("hostname", nodeName),
		attribute.String("genteelrole", genteelRole),
	}

	initGenteelGauges(appName, commonAttribs)
	prometheus := fiberprometheus.NewWithDefaultRegistry(appName)
	prometheus.RegisterAt(app, "/metrics")

	app.Use(prometheus.Middleware)

	app.Use(otelfiber.Middleware())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Genteel Beacon 🚨")
	})

	otlphttpEndpoint, ok := os.LookupEnv("OTLPHTTP_ENDPOINT")
	if ok {
		tp, err := initTracer(otlphttpEndpoint, commonAttribs)
		if err != nil {
			slog.Error("Can't send traces")
		}
		defer func() {
			_ = tp.Shutdown(context.Background())
		}()
		mp, err := initMeter(otlphttpEndpoint, commonAttribs)
		if err != nil {
			log.Fatal("Can't send metrics")
		}
		defer func() {
			_ = mp.Shutdown(context.Background())
		}()
		lp, err := initLogger(otlphttpEndpoint, commonAttribs)
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
		RunGearsmith()
	} else {

		app.Get("/timestamp", func(c *fiber.Ctx) error {
			ctx, span := tracer.Start(c.UserContext(), "TimestampEndpoint")
			span.SetAttributes(attribute.String("RequestID", slogfiber.GetRequestIDFromContext(c.Context())))
			defer span.End()

			if genteelRole != "clock" && genteelRole != "schildwaechter" {
				return fiber.NewError(fiber.StatusBadRequest, "Not my job!")
			}

			greaseErr := greaseGrate(ctx, tracer)
			if greaseErr != nil {
				return greaseErr
			}
			greaseChan <- 1

			theTime := time.Now().Format("2006-01-02 15:04:05")
			return c.SendString(theTime)
		})

		// Define the route for the main path '/telegram'
		app.Get("/telegram", func(c *fiber.Ctx) error {
			ctx, span := tracer.Start(c.UserContext(), "TelegramEndpoint")
			span.SetAttributes(attribute.String("RequestID", slogfiber.GetRequestIDFromContext(c.Context())))
			defer span.End()

			if genteelRole != "telegraphist" && genteelRole != "schildwaechter" {
				return fiber.NewError(fiber.StatusBadRequest, "Not my job!")
			}

			inkErr := inkWell(ctx, tracer)
			if inkErr != nil {
				return inkErr
			}
			inkChan <- 1

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
