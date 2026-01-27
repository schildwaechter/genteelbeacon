// Schildwächter's Genteel Beacon
// Copyright Carsten Thiel 2025-2026
//
// SPDX-Identifier: Apache-2.0

package main

import (
	"context"
	"log"
	"log/slog"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/schildwaechter/genteelbeacon/internal/config"
	"github.com/schildwaechter/genteelbeacon/internal/gearsmith"
	"github.com/schildwaechter/genteelbeacon/internal/handlers"
	"github.com/schildwaechter/genteelbeacon/internal/o11y"
	"github.com/schildwaechter/genteelbeacon/internal/services"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/contrib/otelfiber/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/google/uuid"
	slogfiber "github.com/samber/slog-fiber"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

func main() {
	// initialize ink and grease channels
	services.InitInkGreaseChannels()
	// monitor the ink and grease
	services.StartInkMonitor()
	services.StartGreaseMonitor()
	// refill ink and clean grease
	services.StartInkGreaseTimers()

	app := fiber.New()
	appInt := fiber.New()
	app.Use(requestid.New())
	// telegram background image, before tracing/logging/metrics
	app.Static("/assets/background.png", "./assets/background.png")

	// healthcheck before any tracing/logging/metrics and on internal port
	appInt.Use(healthcheck.New(healthcheck.Config{
		LivenessProbe: func(c *fiber.Ctx) bool {
			if config.GenteelRole == "agitator" {
				// let's agitate
				genteelAgitation, err := strconv.Atoi(config.GetEnv("GENTEEL_AGITATION", "0"))
				if err != nil {
					return false
				}
				if rand.IntN(100) < genteelAgitation {
					return true
				} else {
					return false
				}
			} else {
				return true
			}
		},
		LivenessEndpoint: "/livez",
		ReadinessProbe: func(c *fiber.Ctx) bool {
			if services.GetGreaseBuildup() < 95 && services.GetInkDepletion() < 95 {
				return true
			} else {
				return false
			}
		},
		ReadinessEndpoint: "/readyz",
	}))

	// common attributes for all OTEL data
	commonAttribs := []attribute.KeyValue{
		semconv.ServiceNameKey.String(strings.ToLower(strings.ReplaceAll(config.AppName, " ", ""))),
		semconv.ServiceVersionKey.String(config.BuildVersion),
		semconv.ServiceInstanceIDKey.String(uuid.New().String()),
		attribute.String("hostname", config.NodeName),
		attribute.String("genteelrole", config.GenteelRole),
	}

	// we use both prometheus and OTEL
	o11y.InitGenteelGauges(config.AppName, commonAttribs, services.GetGreaseBuildup, services.GetInkDepletion)
	prometheus := fiberprometheus.NewWithDefaultRegistry(config.AppName)
	prometheus.RegisterAt(appInt, "/metrics")
	app.Use(prometheus.Middleware)
	app.Use(otelfiber.Middleware())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Genteel Beacon 🚨")
	})

	// configure sending OTEL if needed
	// if it's not configured, everything just remains silent
	otlphttpEndpoint, otlphttpOk := os.LookupEnv("OTLPHTTP_ENDPOINT")
	otlphttpTracesEndpoint, otlphttpTracesOk := os.LookupEnv("OTLPHTTP_TRACES_ENDPOINT")
	if otlphttpTracesOk {
		tp, err := o11y.InitTracer(otlphttpTracesEndpoint, commonAttribs)
		if err != nil {
			slog.Error("Can't send traces")
		}
		defer func() {
			_ = tp.Shutdown(context.Background())
		}()
		slog.InfoContext(context.Background(), "Sending traces to "+otlphttpTracesEndpoint)
	} else if otlphttpOk {
		tp, err := o11y.InitTracer(otlphttpEndpoint, commonAttribs)
		if err != nil {
			slog.Error("Can't send traces")
		}
		defer func() {
			_ = tp.Shutdown(context.Background())
		}()
		mp, err := o11y.InitMeter(otlphttpEndpoint, commonAttribs)
		if err != nil {
			log.Fatal("Can't send metrics")
		}
		defer func() {
			_ = mp.Shutdown(context.Background())
		}()
		lp, err := o11y.InitOtelLogger(otlphttpEndpoint, commonAttribs)
		if err != nil {
			log.Fatal("Can't send logs")
		}
		defer func() {
			_ = lp.Shutdown(context.Background())
		}()
		slog.InfoContext(context.Background(), "Sending OTEL data to "+otlphttpEndpoint)
	} else {
		slog.InfoContext(context.Background(), "Not sending OTEL data")
	}
	// set up the logging with fanout to both stdout and (optionally) OTEL
	_, jsonLogging := os.LookupEnv("JSONLOGGING")
	o11y.CreateLogger(config.AppName, jsonLogging)
	// always log traceID, spanID and requestID
	loggerConfig := slogfiber.Config{
		WithSpanID:         true,
		WithTraceID:        true,
		WithRequestID:      true,
		WithRequestHeader:  true,
		WithResponseHeader: true,
	}
	app.Use(slogfiber.NewWithConfig(o11y.Logger, loggerConfig))
	app.Use(recover.New())

	// start tracing now
	// The tracer is accessed via otel.Tracer(config.AppName) throughout the code

	if config.GenteelRole == "gearsmith" {
		gearsmith.RunGearsmith()
	} else {

		handlers.RegisterRoutes(app)
		appPort := config.GetEnv("APP_PORT", "1333")
		appAddr := config.GetEnv("APP_ADDR", "0.0.0.0")
		appIntPort := config.GetEnv("INT_PORT", "1337")
		appIntAddr := config.GetEnv("INT_ADDR", "127.0.0.1")

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Fatal(appInt.Listen(appIntAddr + ":" + appIntPort))
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Fatal(app.Listen(appAddr + ":" + appPort))
		}()
		wg.Wait()
	}
}
