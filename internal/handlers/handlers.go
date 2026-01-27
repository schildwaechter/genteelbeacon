// Schildwächter's Genteel Beacon
// Copyright Carsten Thiel 2025-2026
//
// SPDX-Identifier: Apache-2.0

package handlers

import (
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/schildwaechter/genteelbeacon/internal/config"
	"github.com/schildwaechter/genteelbeacon/internal/o11y"
	"github.com/schildwaechter/genteelbeacon/internal/services"
	"github.com/schildwaechter/genteelbeacon/internal/templates"
	"github.com/schildwaechter/genteelbeacon/internal/types"

	"github.com/enescakir/emoji"
	"github.com/gofiber/fiber/v2"
	slogfiber "github.com/samber/slog-fiber"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

func RegisterRoutes(app *fiber.App) {
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Genteel Beacon 🚨")
	})

	app.Get("/timestamp", func(c *fiber.Ctx) error {
		return handleTimestamp(c)
	})

	app.Get("/telegram", func(c *fiber.Ctx) error {
		return handleTelegram(c)
	})

	app.Get("/emission", func(c *fiber.Ctx) error {
		return handleEmission(c)
	})

	app.Get("/calamity", func(c *fiber.Ctx) error {
		return handleCalamity(c)
	})
}

func handleTimestamp(c *fiber.Ctx) error {
	ctx, span := otel.Tracer(config.AppName).Start(c.UserContext(), "TimestampEndpoint")
	span.SetAttributes(attribute.String("RequestID", slogfiber.GetRequestIDFromContext(c.Context())))
	defer span.End()

	// the binary shall usually one serve a single purpose
	if config.GenteelRole != "clock" && config.GenteelRole != "schildwaechter" {
		return fiber.NewError(fiber.StatusBadRequest, "Not my job!")
	}

	// check whether we have accumulated too much grease
	greaseErr := services.GreaseGrate(ctx)
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
}

func handleTelegram(c *fiber.Ctx) error {
	// add tracing
	ctx, span := otel.Tracer(config.AppName).Start(c.UserContext(), "TelegramEndpoint")
	span.SetAttributes(attribute.String("RequestID", slogfiber.GetRequestIDFromContext(c.Context())))
	defer span.End()

	// the binary shall usually only serve a single purpose
	if config.GenteelRole != "telegraphist" && config.GenteelRole != "schildwaechter" {
		return fiber.NewError(fiber.StatusBadRequest, "Not my job!")
	}

	// test whether we still have ink
	inkErr := services.InkWell(ctx)
	if inkErr != nil {
		return inkErr
	}
	services.InkChan <- 1

	// check whether we use a clock
	var clockResponseData types.ClockReading
	var clockResponseError error = nil
	clock, useClock := os.LookupEnv("GENTEEL_CLOCK")

	if useClock {
		clockResponseData, clockResponseError = services.NimbleCourier(ctx, clock)
	} else {
		// return simplified answer
		o11y.Logger.DebugContext(ctx, "No clock available")
		clockResponseData = types.ClockReading{
			TimeReading: time.Now().Format("2006-01-02"),
			ClockName:   "local",
		}
	}
	if clockResponseError != nil {
		return clockResponseError
	}

	// actually create the message
	clerkMessage, clerkErr := services.DiligentClerk(ctx, clockResponseData, useClock, slogfiber.GetRequestIDFromContext(c.Context()))

	if clerkErr != nil {
		return clerkErr
	}

	// respond with appropriate mimetype
	offer := c.Accepts(fiber.MIMETextPlain, fiber.MIMETextHTML, fiber.MIMEApplicationJSON)
	o11y.Logger.DebugContext(ctx, "Offer: "+offer)
	if offer == "text/html" {
		c.Set("Content-type", "text/html")
		return templates.HtmlTelegram(clerkMessage).Render(c.Context(), c.Response().BodyWriter())
	}
	if offer == "application/json" {
		return c.Status(http.StatusOK).JSON(clerkMessage)
	}
	clerkMessage.Emoji = emoji.Parse(clerkMessage.Emoji)
	tmpl, err := template.New("telegramText").Parse("{{ .Emoji }} {{ .Message }} provided by {{ .ClockReference }}\nBuild {{ .FormVersion }}, »{{ .Service}}« running on {{ .Telegraphist }} 🙋 {{ .Identifier }}")
	if err != nil {
		panic(err)
	}
	return tmpl.ExecuteTemplate(c.Response().BodyWriter(), "telegramText", clerkMessage)
}

func handleEmission(c *fiber.Ctx) error {

	ctx, span := otel.Tracer(config.AppName).Start(c.UserContext(), "EmissionEndpoint")
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
	scribeResponse, _ := services.FocusedScribe(ctx, slogfiber.GetRequestIDFromContext(c.Context()))
	tmpl, err := template.New("callingCardText").Parse("»{{ .Salutation }}« 👩🏻 {{ .Attendant }} 💌 Sincerely, {{ .Signature }}\n✉️ Card version {{ .CardVersion }} 🙋 {{ .Identifier }}")
	if err != nil {
		panic(err)
	}
	// put everything together
	result := map[string]interface{}{
		"Request-Headers":     headers,
		"Genteel-Environment": genteelenvs,
		"Calling-Card":        scribeResponse,
	}
	// respond with appropriate mimetype
	offer := c.Accepts(fiber.MIMETextPlain, fiber.MIMETextHTML, fiber.MIMEApplicationJSON)
	o11y.Logger.DebugContext(ctx, "Offer: "+offer)
	if offer == "application/json" {
		return c.Status(http.StatusOK).JSON(result)
	}
	return tmpl.ExecuteTemplate(c.Response().BodyWriter(), "callingCardText", scribeResponse)
}

func handleCalamity(c *fiber.Ctx) error {
	ctx, span := otel.Tracer(config.AppName).Start(c.UserContext(), "CalamityEndpoint")
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
}
