package main

import (
	"context"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/gofiber/contrib/otelfiber/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	slogfiber "github.com/samber/slog-fiber"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

var buildEpoch string = "0"

// Get environment variable with a default
func getEnv(name string, defaultValue string) string {
	value, exists := os.LookupEnv(name)
	if exists {
		return value
	}
	return defaultValue
}
func InitTracer(tracesEndpoint string, serviceName string) (*trace.TracerProvider, error) {
	ctx := context.Background()
	exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpoint(tracesEndpoint), otlptracehttp.WithInsecure())
	if err != nil {
		return nil, err
	}

	tracerProvider := trace.NewTracerProvider(
		trace.WithSampler(trace.AlwaysSample()),
		trace.WithBatcher(exporter),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName))),
	)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return tracerProvider, nil
}

func main() {
	// our names
	useName := getEnv("USENAME", "Genteel Beacon")
	nodeName, err := os.Hostname()
	if err != nil {
		nodeName = "unknown host"
	}

	tracesEndpoint, ok := os.LookupEnv("OTEL_TRACES_ENDPOINT")
	if ok {
		tp, err := InitTracer(tracesEndpoint, useName)
		if err != nil {
			slog.Error("Can't send traces")
		}
		defer func() {
			_ = tp.Shutdown(context.Background())
		}()
		slog.Info("Sending traces to " + tracesEndpoint)
	} else {
		slog.Info("Not sending traces")
	}

	app := fiber.New()
	app.Use(requestid.New())
	app.Use(otelfiber.Middleware())

	_, jsonLogging := os.LookupEnv("JSONLOGGING")
	var logger *slog.Logger
	if jsonLogging {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}
	//slog.SetDefault(logger)
	app.Use(slogfiber.New(logger))
	app.Use(recover.New())

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

	client := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	// Define a route for the root path '/'
	app.Get("/", func(c *fiber.Ctx) error {
		tracer := otel.Tracer("root")
		ctx, span := tracer.Start(c.UserContext(), "Root Endpoint")
		span.SetAttributes(attribute.String("RequestID", slogfiber.GetRequestIDFromContext(c.Context())))
		defer span.End()

		var backendResponseString string
		var backendResponse bool = false
		backend, ok := os.LookupEnv("BACKEND")
		if ok {

			req, err := http.NewRequestWithContext(ctx, "GET", backend, nil)
			req.Header.Set("X-Request-Id", slogfiber.GetRequestIDFromContext(c.Context()))

			// Inject TraceParent to Context
			otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

			for k, v := range req.Header {
				slog.Info("%s: %s\n", k, v)
			}

			resp, err := client.Do(req)
			if err != nil {
				span.RecordError(err)
				return c.Status(http.StatusInternalServerError).SendString("Error calling backend!")
			}
			defer resp.Body.Close()

			responseData, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Fatal(err)
			}
			backendResponse = true
			backendResponseString = string(responseData)

		} else {
			slog.Debug("No backend defined")
		}

		if backendResponse {
			return c.SendString("Build time: " + buildEpoch + ", »" + useName + "« running on " + nodeName + " 🙋 " + slogfiber.GetRequestIDFromContext(c.Context()) + " 📫 " + backendResponseString)

		} else {
			return c.SendString("Build time: " + buildEpoch + ", »" + useName + "« running on " + nodeName + " 🙋 " + slogfiber.GetRequestIDFromContext(c.Context()) + " 🏁")
		}
	})

	// Start the server on the specified port
	runPort := getEnv("RUNPORT", "1333")
	log.Fatal(app.Listen(":" + runPort))
}
