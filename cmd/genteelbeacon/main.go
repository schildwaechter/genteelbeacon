// Schildwächter's Genteel Beacon
// Copyright Carsten Thiel 2025
//
// SPDX-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/schildwaechter/genteelbeacon/internal/templates"
	"github.com/schildwaechter/genteelbeacon/internal/types"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/enescakir/emoji"
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
	// to be overwritten on build
	buildVersion string = "0.0.0"
	// grease and ink tracking
	greaseBuildup int64 = 0
	inkDepletion  int64 = 0
	greaseChan    chan int64
	inkChan       chan int64

	greaseBuildupGaugeProm prometheus.Gauge
	inkDepletionGaugeProm  prometheus.Gauge
	greaseBuildupGaugeOtel metric.Int64ObservableGauge
	inkDepletionGaugeOtel  metric.Int64ObservableGauge

	tracer trace.Tracer
	logger *slog.Logger
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

// check whether greas buildup is too much
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
		// this is a serious failure
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

// check whether we have depelted the ink
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
		// this is a serious failure
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

// create the telegram to be sent
func scribeStudy(ctx context.Context, tracer trace.Tracer, appName string, clockResponseData types.ClockReading, useClock bool, requestId string) (types.Telegram, error) {
	ctx, span := tracer.Start(ctx, "ScribeStudy")
	defer span.End()

	logger.DebugContext(ctx, "Scribe at work 🖊️")

	nodeName, err := os.Hostname()
	if err != nil {
		nodeName = "unknown host"
	}

	span.AddEvent("Preparing message")
	scribeErrorChance := rand.Float64()
	var responseTelegram types.Telegram
	responseTelegram.Identifier = requestId
	responseTelegram.Service = appName
	responseTelegram.Telegraphist = nodeName
	responseTelegram.FormVersion = buildVersion
	if useClock {
		responseTelegram.Message = "The time is " + clockResponseData.TimeReading
		responseTelegram.Emoji = ":mantelpiece_clock:"
		responseTelegram.ClockReference = clockResponseData.ClockName
	} else {
		responseTelegram.Message = "Today is " + clockResponseData.TimeReading + " – that's all we have!"
		responseTelegram.Emoji = ":calendar:"
		responseTelegram.ClockReference = "unavailable"
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

		responseTelegram.Message = "The time is not available at this moment!!"
		return responseTelegram, fiber.NewError(fiber.StatusTeapot, err.Error())
	} else if scribeErrorChance > 0.96 { // oh dear (if we haven't tripped before)
		span.AddEvent("Urgent need")
		err := errors.New("Scribe seems to be indisposed 💩")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		logger.ErrorContext(ctx, err.Error(), loggerTraceAttr(ctx, span), loggerSpanAttr(ctx, span))

		responseTelegram.Message = "The time is not available at this moment!!"
		return responseTelegram, fiber.NewError(fiber.StatusServiceUnavailable, err.Error())
	} else {
		time.Sleep(time.Duration(rand.IntN(70)+1) * time.Millisecond) // normal artificial span increase
		span.AddEvent("Message ready")
	}

	return responseTelegram, nil
}

func drawingRoom(ctx context.Context, tracer trace.Tracer, appName string, requestId string) (types.CallingCard, error) {
	ctx, span := tracer.Start(ctx, "DrawingRoom")
	defer span.End()

	var responseCallingCard types.CallingCard
	nodeName, err := os.Hostname()
	if err != nil {
		nodeName = "unknown host"
	}
	nodOptions := []string{
		"A pleasure!", "Charmed!", "Delighted!", "Charmed, I'm sure!",
		"Quite so!", "Splendid!", "How lovely!", "My compliments!",
		"Pray tell!", "Fancy that!", "Always a joy!", "Quel plaisir!",
		"Enchantée!", "Très honorée!", "Très ravie!",
	}
	randomIndex := rand.IntN(len(nodOptions))
	randomNod := nodOptions[randomIndex]
	responseCallingCard.Attendant = appName
	responseCallingCard.Salutation = randomNod
	responseCallingCard.CardVersion = buildVersion
	responseCallingCard.Signature = nodeName
	responseCallingCard.Identifier = requestId

	return responseCallingCard, nil
}

// check the remote clock
func courteousCourier(ctx context.Context, tracer trace.Tracer, client *http.Client, clock string) (error, types.ClockReading) {
	_, span := tracer.Start(ctx, "CourteousCourier")
	defer span.End()

	logger.DebugContext(ctx, "Courier checking "+clock+" 🐦", loggerTraceAttr(ctx, span), loggerSpanAttr(ctx, span))

	req, err := http.NewRequestWithContext(ctx, "GET", clock+"/timestamp", nil)

	// Inject TraceParent to Context
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
	var ClockResponseData types.ClockReading

	resp, err := client.Do(req)
	if err != nil {
		span.RecordError(err)
		logger.ErrorContext(ctx, "Error checking clock!", loggerTraceAttr(ctx, span), loggerSpanAttr(ctx, span))
		ClockResponseData = types.ClockReading{
			TimeReading: "Error checking clock!",
			ClockName:   "unknown",
		}

		return err, ClockResponseData
	}
	defer resp.Body.Close()

	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		span.RecordError(err)
		logger.ErrorContext(ctx, err.Error())
		ClockResponseData = types.ClockReading{
			TimeReading: err.Error(),
			ClockName:   "unknown",
		}
		return nil, ClockResponseData
	}
	json.Unmarshal(responseData, &ClockResponseData)

	return nil, ClockResponseData
}

// set up metrics in both OTEL and Proemtheus
func initGenteelGauges(appName string, commonAttribs []attribute.KeyValue) error {
	meterProvider := otel.GetMeterProvider()
	meter := meterProvider.Meter(appName)

	// register the OTEL metrics
	greaseBuildupGaugeOtel, _ = meter.Int64ObservableGauge(
		"genteelbeacon_greasebuildup",
		metric.WithDescription("The Genteel Beacon's current grease buildup"),
	)
	inkDepletionGaugeOtel, _ = meter.Int64ObservableGauge(
		"genteelbeacon_inkdepletion",
		metric.WithDescription("The Genteel Beacon's current ink depletion"),
	)

	promLabels := make(prometheus.Labels)
	for _, attr := range commonAttribs {
		promLabels[string(attr.Key)] = attr.Value.AsString()
	}

	// register the Prometheus metrics
	greaseBuildupGaugeProm = promauto.NewGauge(prometheus.GaugeOpts{
		Name:        "genteelbeacon_greasebuildup_p",
		Help:        "The Genteel Beacon's current grease buidlup",
		ConstLabels: promLabels,
	})
	inkDepletionGaugeProm = promauto.NewGauge(prometheus.GaugeOpts{
		Name:        "genteelbeacon_inkdepletion_p",
		Help:        "The Genteel Beacon's current ink depletion",
		ConstLabels: promLabels,
	})

	// OTEL sending as callback on meter activity (from the channel handlers)
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

	// manage the grease and ink
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

	// refill ink and clean grease
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			greaseChan <- -1
			inkChan <- -1
		}
	}()

	app := fiber.New()
	appInt := fiber.New()
	app.Use(requestid.New())
	// telegram background image, before tracing/logging/metrics
	app.Static("/assets/background.png", "./assets/background.png")

	// healthcheck before any tracing/logging/metrics and on internal port
	appInt.Use(healthcheck.New(healthcheck.Config{
		LivenessProbe: func(c *fiber.Ctx) bool {
			if genteelRole == "agitator" {
				// let's agitate
				genteelAgitation, err := strconv.Atoi(getEnv("GENTEEL_AGITATION", "0"))
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
			if greaseBuildup < 95 && inkDepletion < 95 {
				return true
			} else {
				return false
			}
		},
		ReadinessEndpoint: "/readyz",
	}))

	// common attributes for all OTEL data
	commonAttribs := []attribute.KeyValue{
		semconv.ServiceNameKey.String(strings.ToLower(strings.ReplaceAll(appName, " ", ""))),
		semconv.ServiceInstanceIDKey.String(uuid.New().String()),
		attribute.String("hostname", nodeName),
		attribute.String("genteelrole", genteelRole),
	}

	// we use both prometheus and OTEL
	initGenteelGauges(appName, commonAttribs)
	prometheus := fiberprometheus.NewWithDefaultRegistry(appName)
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
		tp, err := initTracer(otlphttpTracesEndpoint, commonAttribs)
		if err != nil {
			slog.Error("Can't send traces")
		}
		defer func() {
			_ = tp.Shutdown(context.Background())
		}()
		slog.InfoContext(context.Background(), "Sending traces to "+otlphttpTracesEndpoint)
	} else if otlphttpOk {
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
		slog.InfoContext(context.Background(), "Sending OTEL data to "+otlphttpEndpoint)
	} else {
		slog.InfoContext(context.Background(), "Not sending OTEL data")
	}
	// set up the logging with fanout to both stdout and (optionally) OTEL
	_, jsonLogging := os.LookupEnv("JSONLOGGING")
	if jsonLogging {
		logger = slog.New(
			slogmulti.Fanout(
				otelslog.NewLogger(appName).Handler(),
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
	// always log traceID, spanID and requestID
	loggerConfig := slogfiber.Config{
		WithSpanID:         true,
		WithTraceID:        true,
		WithRequestID:      true,
		WithRequestHeader:  true,
		WithResponseHeader: true,
	}
	app.Use(slogfiber.NewWithConfig(logger, loggerConfig))
	app.Use(recover.New())

	// we need to make calls out
	client := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	// start tracing now
	tracer = otel.Tracer(appName)

	if genteelRole == "gearsmith" {
		RunGearsmith()
	} else {

		app.Get("/timestamp", func(c *fiber.Ctx) error {
			ctx, span := tracer.Start(c.UserContext(), "TimestampEndpoint")
			span.SetAttributes(attribute.String("RequestID", slogfiber.GetRequestIDFromContext(c.Context())))
			defer span.End()

			// the binary shall usually one serve a single purpose
			if genteelRole != "clock" && genteelRole != "schildwaechter" {
				return fiber.NewError(fiber.StatusBadRequest, "Not my job!")
			}

			// check whether we have accumulated too much grease
			greaseErr := greaseGrate(ctx, tracer)
			if greaseErr != nil {
				return greaseErr
			}
			greaseChan <- 1

			// prepare the answer with hostname and current time
			nodeName, err := os.Hostname()
			if err != nil {
				nodeName = "unknown host"
			}
			myClockReading := types.ClockReading{
				TimeReading: time.Now().Format("2006-01-02 15:04:05"),
				ClockName:   nodeName,
			}

			return c.Status(http.StatusOK).JSON(myClockReading)
		})

		app.Get("/telegram", func(c *fiber.Ctx) error {
			// add tracing
			ctx, span := tracer.Start(c.UserContext(), "TelegramEndpoint")
			span.SetAttributes(attribute.String("RequestID", slogfiber.GetRequestIDFromContext(c.Context())))
			defer span.End()

			// the binary shall usually only serve a single purpose
			if genteelRole != "telegraphist" && genteelRole != "schildwaechter" {
				return fiber.NewError(fiber.StatusBadRequest, "Not my job!")
			}

			// test whether we still have ink
			inkErr := inkWell(ctx, tracer)
			if inkErr != nil {
				return inkErr
			}
			inkChan <- 1

			// check whether we use a clock
			var ClockResponseData types.ClockReading
			var clockResponseError error = nil
			clock, useClock := os.LookupEnv("GENTEEL_CLOCK")

			if useClock {
				clockResponseError, ClockResponseData = courteousCourier(ctx, tracer, client, clock)
			} else {
				// return simplified answer
				logger.DebugContext(ctx, "No clock available")
				ClockResponseData = types.ClockReading{
					TimeReading: time.Now().Format("2006-01-02"),
					ClockName:   "local",
				}
			}
			if clockResponseError != nil {
				return clockResponseError
			}

			// actually create the message
			scribeStudyMessage, scribeErr := scribeStudy(ctx, tracer, appName, ClockResponseData, useClock, slogfiber.GetRequestIDFromContext(c.Context()))

			if scribeErr != nil {
				return scribeErr
			}

			// respond with appropriate mimetype
			offer := c.Accepts(fiber.MIMETextPlain, fiber.MIMETextHTML, fiber.MIMEApplicationJSON)
			logger.DebugContext(ctx, "Offer: "+offer)
			if offer == "text/html" {
				c.Set("Content-type", "text/html")
				return templates.HtmlTelegram(scribeStudyMessage).Render(c.Context(), c.Response().BodyWriter())
			}
			if offer == "application/json" {
				return c.Status(http.StatusOK).JSON(scribeStudyMessage)
			}
			scribeStudyMessage.Emoji = emoji.Parse(scribeStudyMessage.Emoji)
			tmpl, err := template.New("telegramText").Parse("{{ .Emoji }} {{ .Message }} provided by {{ .ClockReference }}\nBuild {{ .FormVersion }}, »{{ .Service}}« running on {{ .Telegraphist }} 🙋 {{ .Identifier }}")
			if err != nil {
				panic(err)
			}
			return tmpl.ExecuteTemplate(c.Response().BodyWriter(), "telegramText", scribeStudyMessage)
		})

		app.Get("/emission", func(c *fiber.Ctx) error {
			ctx, span := tracer.Start(c.UserContext(), "EmissionEndpoint")
			span.SetAttributes(attribute.String("RequestID", slogfiber.GetRequestIDFromContext(c.Context())))
			defer span.End()
			// the binary shall usually only serve a single purpose
			if genteelRole != "lightkeeper" && genteelRole != "schildwaechter" {
				return fiber.NewError(fiber.StatusBadRequest, "Not my job!")
			}
			logger.InfoContext(ctx, "Emanating local information with request headers", loggerTraceAttr(ctx, span), loggerSpanAttr(ctx, span))
			headers := make(map[string]string)
			c.Request().Header.VisitAll(func(key, value []byte) {
				headers[string(key)] = string(value)
			})
			// gather Genteel environment variables
			genteelenvs := make(map[string]string)
			for _, e := range os.Environ() {
				if strings.HasPrefix(strings.ToUpper(e), "GENTEEL_") {
					pair := strings.SplitN(e, "=", 2)
					if len(pair) == 2 {
						genteelenvs[pair[0]] = pair[1]
					}
				}
			}
			// get calling card
			drawingRoomResponse, _ := drawingRoom(ctx, tracer, appName, slogfiber.GetRequestIDFromContext(c.Context()))
			tmpl, err := template.New("callingCardText").Parse("»{{ .Salutation }}« 👩🏻 {{ .Attendant }} 💌 Sincerely, {{ .Signature }}\n✉️ Card version {{ .CardVersion }} 🙋 {{ .Identifier }}")
			if err != nil {
				panic(err)
			}
			// put everything together
			result := map[string]interface{}{
				"Request-Headers":     headers,
				"Genteel-Environment": genteelenvs,
				"Calling-Card":        drawingRoomResponse,
			}
			// respond with appropriate mimetype
			offer := c.Accepts(fiber.MIMETextPlain, fiber.MIMETextHTML, fiber.MIMEApplicationJSON)
			logger.DebugContext(ctx, "Offer: "+offer)
			if offer == "application/json" {
				return c.Status(http.StatusOK).JSON(result)
			}
			return tmpl.ExecuteTemplate(c.Response().BodyWriter(), "callingCardText", drawingRoomResponse)
		})

		app.Get("/calamity", func(c *fiber.Ctx) error {
			ctx, span := tracer.Start(c.UserContext(), "CalamityEndpoint")
			span.SetAttributes(attribute.String("RequestID", slogfiber.GetRequestIDFromContext(c.Context())))
			defer span.End()
			// the binary shall usually only serve a single purpose
			if genteelRole == "agitator" {
				// this is a bit brutal
				logger.ErrorContext(ctx, "Disrupt!", loggerTraceAttr(ctx, span), loggerSpanAttr(ctx, span))
				os.Exit(133)
			}
			if genteelRole != "lightkeeper" && genteelRole != "schildwaechter" {
				return fiber.NewError(fiber.StatusBadRequest, "Not my job!")
			}
			// causing an error on purpose
			logger.ErrorContext(ctx, "Calamity has been invoked!", loggerTraceAttr(ctx, span), loggerSpanAttr(ctx, span))
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"message": "Oh, no! A most dreadful calamity has occured! 💥"})
		})

		appPort := getEnv("APP_PORT", "1333")
		appAddr := getEnv("APP_ADDR", "0.0.0.0")
		appIntPort := getEnv("INT_PORT", "1337")
		appIntAddr := getEnv("INT_ADDR", "127.0.0.1")

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
