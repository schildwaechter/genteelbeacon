package main

import (
	"context"
	"errors"
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

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	otellog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/contrib/bridges/otelslog"
)

var (
	buildEpoch string = "0"
	tracer     trace.Tracer
)

// Get environment variable with a default
func getEnv(name string, defaultValue string) string {
	value, exists := os.LookupEnv(name)
	if exists {
		return value
	}
	return defaultValue
}
func InitTracer(otlphttpEndpoint string, serviceName string) (*sdktrace.TracerProvider, error) {
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

func InitMeter(otlphttpEndpoint string, serviceName string) (*metric.MeterProvider, error) {
	ctx := context.Background()
	metricExporter, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpoint(otlphttpEndpoint), otlpmetrichttp.WithInsecure())
	if err != nil {
		return nil, err
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter)),
		metric.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)),
	)
	otel.SetMeterProvider(meterProvider)

	return meterProvider, nil
}

func InitLogger(otlphttpEndpoint string, serviceName string) (*otellog.LoggerProvider, error) {
	ctx := context.Background()
	logExporter, err := otlploghttp.New(ctx, otlploghttp.WithEndpoint(otlphttpEndpoint), otlploghttp.WithInsecure())
	if err != nil {
		return nil, err
	}

	logProvider := otellog.NewLoggerProvider(
		otellog.WithProcessor(otellog.NewBatchProcessor(logExporter)),
		otellog.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)),
	)
	global.SetLoggerProvider(logProvider)

	return logProvider, nil
}

func greaseGrate(ctx context.Context, tracer trace.Tracer) error {
	_, span := tracer.Start(ctx, "Grease Grate")
	defer span.End()

	if rand.Float64() > 0.95 {
		time.Sleep(8 * time.Millisecond) // artifical slowdown
		err := errors.New("Grease Issue!")
		//span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		slog.Error(err.Error())
		return err
	} else {
		time.Sleep(1 * time.Millisecond) // artifical slowdown
	}

	return nil
}

func scribeStudy(ctx context.Context, tracer trace.Tracer, useName string, requestId string) string {
	_, span := tracer.Start(ctx, "Scribe Study")
	defer span.End()

	nodeName, err := os.Hostname()
	if err != nil {
		nodeName = "unknown host"
	}

	time.Sleep(time.Duration(rand.IntN(100)+1) * time.Millisecond) // artifical slowdown
	return "Build time: " + buildEpoch + ", »" + useName + "« running on " + nodeName + " 🙋 " + requestId
}

func courteousCourier(ctx context.Context, tracer trace.Tracer, client *http.Client, backend string) (bool, string) {
	_, span := tracer.Start(ctx, "CourteousCourier")
	defer span.End()

	req, err := http.NewRequestWithContext(ctx, "GET", backend+"/telegram", nil)

	// Inject TraceParent to Context
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := client.Do(req)
	if err != nil {
		span.RecordError(err)
		slog.ErrorContext(ctx, "Error calling backend!")
		return false, "Error calling backend!"
	}
	defer resp.Body.Close()

	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		span.RecordError(err)
		log.Fatal(err)
		return false, ""
	}

	return true, string(responseData)
}

func main() {
	// our names
	useName := getEnv("USENAME", "Genteel Beacon")

	app := fiber.New()
	app.Use(requestid.New())

	app.Use(healthcheck.New(healthcheck.Config{
		LivenessProbe: func(c *fiber.Ctx) bool {
			return true
		},
		LivenessEndpoint: "/livez",
		ReadinessProbe: func(c *fiber.Ctx) bool {
			return true
		},
		ReadinessEndpoint: "/readyz",
	}))

	app.Use(otelfiber.Middleware())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Genteel Beacon 🚨")
	})

	otlphttpEndpoint, ok := os.LookupEnv("OTLPHTTP_ENDPOINT")
	if ok {
		tp, err := InitTracer(otlphttpEndpoint, useName)
		if err != nil {
			slog.Error("Can't send traces")
		}
		defer func() {
			_ = tp.Shutdown(context.Background())
		}()
		mp, err := InitMeter(otlphttpEndpoint, useName)
		if err != nil {
			log.Fatal("Can't send metrics")
		}
		defer func() {
			_ = mp.Shutdown(context.Background())
		}()
		lp, err := InitLogger(otlphttpEndpoint, useName)
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
	prometheus := fiberprometheus.New(useName)
	prometheus.RegisterAt(app, "/metrics")

	app.Use(prometheus.Middleware)
	//slog.SetDefault(otelslog.NewLogger(useName))

	_, jsonLogging := os.LookupEnv("JSONLOGGING")
	var logger *slog.Logger
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
				otelslog.NewLogger(useName).Handler(),
				//slog.Default().Handler(),
				slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}),
			),
		)
	}
	//slog.SetDefault(logger)
	app.Use(slogfiber.New(logger))
	app.Use(recover.New())

	client := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	tracer = otel.Tracer("my tracer")

	// Define a route for the root path '/'
	app.Get("/telegram", func(c *fiber.Ctx) error {
		ctx, span := tracer.Start(c.UserContext(), "Telegram Endpoint")
		span.SetAttributes(attribute.String("RequestID", slogfiber.GetRequestIDFromContext(c.Context())))
		defer span.End()

		greaseGrate(ctx, tracer)

		var backendResponseString string
		var backendResponse bool = false
		backend, ok := os.LookupEnv("BACKEND")
		if ok {

			backendResponse, backendResponseString = courteousCourier(ctx, tracer, client, backend)

		} else {
			slog.Debug("No backend defined")
		}

		scribeStudyMessage := scribeStudy(ctx, tracer, useName, slogfiber.GetRequestIDFromContext(c.Context()))
		if backendResponse {
			return c.SendString(scribeStudyMessage + " 📫 " + backendResponseString)

		} else {
			return c.SendString(scribeStudyMessage + " 🏁")
		}
	})

	// Start the server on the specified port and address
	appPort := getEnv("APP_PORT", "1333")
	appAddr := getEnv("APP_ADDR", "0.0.0.0")
	log.Fatal(app.Listen(appAddr + ":" + appPort))
}
