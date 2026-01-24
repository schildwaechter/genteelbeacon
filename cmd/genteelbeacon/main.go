// Schildwächter's Genteel Beacon
// Copyright Carsten Thiel 2025
//
// SPDX-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
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

	"github.com/schildwaechter/genteelbeacon/internal/config"
	"github.com/schildwaechter/genteelbeacon/internal/gearsmith"
	"github.com/schildwaechter/genteelbeacon/internal/o11y"
	"github.com/schildwaechter/genteelbeacon/internal/services"
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

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	// to be overwritten on build
	buildVersion string = "0.0.0"
	// grease and ink tracking
	greaseBuildup int64 = 0
	inkDepletion  int64 = 0

	tracer trace.Tracer
)

// Get environment variable with a default
func getEnv(name string, defaultValue string) string {
	value, exists := os.LookupEnv(name)
	if exists {
		return value
	}
	return defaultValue
}

// create the telegram to be sent
func scribeStudy(ctx context.Context, tracer trace.Tracer, appName string, clockResponseData types.ClockReading, useClock bool, requestId string) (types.Telegram, error) {
	ctx, span := tracer.Start(ctx, "ScribeStudy")
	defer span.End()

	o11y.Logger.DebugContext(ctx, "Scribe at work 🖊️")

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
		o11y.Logger.WarnContext(ctx, "Scribe dropped the pen 🔍!!", o11y.LoggerTraceAttr(ctx, span), o11y.LoggerSpanAttr(ctx, span))
		time.Sleep(3 * time.Second) // uppss...
	} else if scribeErrorChance > 0.99 { // somestimes it can't wait
		span.AddEvent("Break time")
		err := errors.New("Scribe seems to be having a break 🫖")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		o11y.Logger.ErrorContext(ctx, err.Error(), o11y.LoggerTraceAttr(ctx, span), o11y.LoggerSpanAttr(ctx, span))

		responseTelegram.Message = "The time is not available at this moment!!"
		return responseTelegram, fiber.NewError(fiber.StatusTeapot, err.Error())
	} else if scribeErrorChance > 0.96 { // oh dear (if we haven't tripped before)
		span.AddEvent("Urgent need")
		err := errors.New("Scribe seems to be indisposed 💩")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		o11y.Logger.ErrorContext(ctx, err.Error(), o11y.LoggerTraceAttr(ctx, span), o11y.LoggerSpanAttr(ctx, span))

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

	o11y.Logger.DebugContext(ctx, "Courier checking "+clock+" 🐦", o11y.LoggerTraceAttr(ctx, span), o11y.LoggerSpanAttr(ctx, span))

	req, err := http.NewRequestWithContext(ctx, "GET", clock+"/timestamp", nil)

	// Inject TraceParent to Context
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
	var ClockResponseData types.ClockReading

	resp, err := client.Do(req)
	if err != nil {
		span.RecordError(err)
		o11y.Logger.ErrorContext(ctx, "Error checking clock!", o11y.LoggerTraceAttr(ctx, span), o11y.LoggerSpanAttr(ctx, span))
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
		o11y.Logger.ErrorContext(ctx, err.Error())
		ClockResponseData = types.ClockReading{
			TimeReading: err.Error(),
			ClockName:   "unknown",
		}
		return nil, ClockResponseData
	}
	json.Unmarshal(responseData, &ClockResponseData)

	return nil, ClockResponseData
}

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
		semconv.ServiceNameKey.String(strings.ToLower(strings.ReplaceAll(config.AppName, " ", ""))),
		semconv.ServiceVersionKey.String(buildVersion),
		semconv.ServiceInstanceIDKey.String(uuid.New().String()),
		attribute.String("hostname", config.NodeName),
		attribute.String("genteelrole", config.GenteelRole),
	}

	// we use both prometheus and OTEL
	o11y.InitGenteelGauges(config.AppName, commonAttribs, &greaseBuildup, &inkDepletion)
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

	// we need to make calls out
	client := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	// start tracing now
	tracer = otel.Tracer(config.AppName)

	if config.GenteelRole == "gearsmith" {
		gearsmith.RunGearsmith()
	} else {

		app.Get("/timestamp", func(c *fiber.Ctx) error {
			ctx, span := tracer.Start(c.UserContext(), "TimestampEndpoint")
			span.SetAttributes(attribute.String("RequestID", slogfiber.GetRequestIDFromContext(c.Context())))
			defer span.End()

			// the binary shall usually one serve a single purpose
			if config.GenteelRole != "clock" && config.GenteelRole != "schildwaechter" {
				return fiber.NewError(fiber.StatusBadRequest, "Not my job!")
			}

			// check whether we have accumulated too much grease
			greaseErr := services.GreaseGrate(ctx, tracer)
			if greaseErr != nil {
				return greaseErr
			}
			services.GreaseChan <- 1

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
			if config.GenteelRole != "telegraphist" && config.GenteelRole != "schildwaechter" {
				return fiber.NewError(fiber.StatusBadRequest, "Not my job!")
			}

			// test whether we still have ink
			inkErr := services.InkWell(ctx, tracer)
			if inkErr != nil {
				return inkErr
			}
			services.InkChan <- 1

			// check whether we use a clock
			var ClockResponseData types.ClockReading
			var clockResponseError error = nil
			clock, useClock := os.LookupEnv("GENTEEL_CLOCK")

			if useClock {
				clockResponseError, ClockResponseData = courteousCourier(ctx, tracer, client, clock)
			} else {
				// return simplified answer
				o11y.Logger.DebugContext(ctx, "No clock available")
				ClockResponseData = types.ClockReading{
					TimeReading: time.Now().Format("2006-01-02"),
					ClockName:   "local",
				}
			}
			if clockResponseError != nil {
				return clockResponseError
			}

			// actually create the message
			scribeStudyMessage, scribeErr := scribeStudy(ctx, tracer, config.AppName, ClockResponseData, useClock, slogfiber.GetRequestIDFromContext(c.Context()))

			if scribeErr != nil {
				return scribeErr
			}

			// respond with appropriate mimetype
			offer := c.Accepts(fiber.MIMETextPlain, fiber.MIMETextHTML, fiber.MIMEApplicationJSON)
			o11y.Logger.DebugContext(ctx, "Offer: "+offer)
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
			if config.GenteelRole != "lightkeeper" && config.GenteelRole != "schildwaechter" {
				return fiber.NewError(fiber.StatusBadRequest, "Not my job!")
			}
			o11y.Logger.InfoContext(ctx, "Emanating local information with request headers", o11y.LoggerTraceAttr(ctx, span), o11y.LoggerSpanAttr(ctx, span))
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
			drawingRoomResponse, _ := drawingRoom(ctx, tracer, config.AppName, slogfiber.GetRequestIDFromContext(c.Context()))
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
			o11y.Logger.DebugContext(ctx, "Offer: "+offer)
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
			if config.GenteelRole == "agitator" {
				// this is a bit brutal
				o11y.Logger.ErrorContext(ctx, "Disrupt!", o11y.LoggerTraceAttr(ctx, span), o11y.LoggerSpanAttr(ctx, span))
				os.Exit(133)
			}
			if config.GenteelRole != "lightkeeper" && config.GenteelRole != "schildwaechter" {
				return fiber.NewError(fiber.StatusBadRequest, "Not my job!")
			}
			// causing an error on purpose
			o11y.Logger.ErrorContext(ctx, "Calamity has been invoked!", o11y.LoggerTraceAttr(ctx, span), o11y.LoggerSpanAttr(ctx, span))
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
