// Schildwächter's Genteel Beacon
// Copyright Carsten Thiel 2025-2026
//
// SPDX-Identifier: Apache-2.0

package services

import (
	"context"
	"errors"
	"math/rand/v2"
	"os"
	"time"

	"github.com/schildwaechter/genteelbeacon/internal/config"
	"github.com/schildwaechter/genteelbeacon/internal/o11y"
	"github.com/schildwaechter/genteelbeacon/internal/types"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

// DiligentClerk creates the telegram to be sent
func DiligentClerk(ctx context.Context, clockResponseData types.ClockReading, useClock bool, requestID string) (types.Telegram, error) {
	ctx, span := otel.Tracer(config.AppName).Start(ctx, "DiligentClerk")
	defer span.End()

	o11y.Logger.DebugContext(ctx, "Clerk at work 🖊️")

	nodeName, err := os.Hostname()
	if err != nil {
		nodeName = "unknown host"
	}

	span.AddEvent("Preparing message")
	clerkRandErrChance1 := rand.Float64()
	clerkRandErrChance2 := rand.Float64()
	var responseTelegram types.Telegram
	responseTelegram.Identifier = requestID
	responseTelegram.Service = config.AppName
	responseTelegram.Telegraphist = nodeName
	responseTelegram.FormVersion = config.BuildVersion
	if useClock {
		responseTelegram.Message = "The time is " + clockResponseData.TimeReading
		responseTelegram.Emoji = ":mantelpiece_clock:"
		responseTelegram.ClockReference = clockResponseData.ClockName
	} else {
		responseTelegram.Message = "Today is " + clockResponseData.TimeReading + " – that's all we have!"
		responseTelegram.Emoji = ":calendar:"
		responseTelegram.ClockReference = "unavailable"
	}

	if clerkRandErrChance1 < config.GetChaosChance("breakChance") { // somestimes it can't wait
		span.AddEvent("Break time")
		err := errors.New("clerk seems to be having a break 🫖")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		o11y.Logger.ErrorContext(ctx, err.Error(), o11y.LoggerTraceAttr(ctx, span), o11y.LoggerSpanAttr(ctx, span))

		responseTelegram.Message = "The time is not available at this moment!!"
		return responseTelegram, fiber.NewError(fiber.StatusTeapot, err.Error())
	} else if clerkRandErrChance2 < config.GetChaosChance("indisposedChance") { // oh dear (if we haven't tripped before)
		span.AddEvent("Urgent need")
		err := errors.New("clerk seems to be indisposed 💩")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		o11y.Logger.ErrorContext(ctx, err.Error(), o11y.LoggerTraceAttr(ctx, span), o11y.LoggerSpanAttr(ctx, span))

		responseTelegram.Message = "The time is not available at this moment!!"
		return responseTelegram, fiber.NewError(fiber.StatusServiceUnavailable, err.Error())
	} else {
		time.Sleep(time.Duration(rand.IntN(70)+20) * time.Millisecond) // normal artificial span increase
		span.AddEvent("Message ready")
	}

	return responseTelegram, nil
}
