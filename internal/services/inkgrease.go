// Schildwächter's Genteel Beacon
// Copyright Carsten Thiel 2025
//
// SPDX-Identifier: Apache-2.0

package services

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/schildwaechter/genteelbeacon/internal/o11y"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var (
	// grease and ink tracking
	GreaseBuildup int64 = 0
	InkDepletion  int64 = 0
	GreaseChan    chan int64
	InkChan       chan int64
)

// InitInkGreaseChannels initializes the grease and ink channels
func InitInkGreaseChannels() {
	GreaseChan = make(chan int64)
	InkChan = make(chan int64)
}

// StartGreaseMonitor manages the grease buildup
func StartGreaseMonitor() {
	go func() {
		for {
			greaseChange := <-GreaseChan
			if greaseChange == -1 && GreaseBuildup > 0 {
				GreaseBuildup--
				o11y.GreaseBuildupGaugeProm.Dec()
			} else if greaseChange == 1 && rand.IntN(100) < 50 {
				GreaseBuildup++
				o11y.GreaseBuildupGaugeProm.Inc()
			}
		}
	}()
}

// StartInkMonitor manages the ink depletion
func StartInkMonitor() {
	go func() {
		for {
			inkChange := <-InkChan
			if inkChange == -1 && InkDepletion > 0 {
				InkDepletion--
				o11y.InkDepletionGaugeProm.Dec()
			} else if inkChange == 1 {
				InkDepletion++
				o11y.InkDepletionGaugeProm.Inc()
			}
		}
	}()
}

// StartInkGreaseTimers start a job to refill ink and cleans grease periodically
func StartInkGreaseTimers() {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			GreaseChan <- -1
			InkChan <- -1
		}
	}()
}

// GreaseGrate checks whether grease buildup is too much
func GreaseGrate(ctx context.Context, tracer trace.Tracer) error {
	childCtx, span := tracer.Start(ctx, "GreaseGrate")
	defer span.End()

	// Whether to trip (between 0 and 1)
	tripValue := rand.Float64()
	// The threshold to trip the grease grate:
	// not below 0.9, increasing probablility from 0.9-1 and always above
	tripThreshold := float64(GreaseBuildup-90) / 10

	o11y.Logger.DebugContext(childCtx, fmt.Sprintf("greaseBuildup %d - tripThreshold %f - tripValue %f", GreaseBuildup, tripThreshold, tripValue))

	if tripValue < tripThreshold {
		// this is a serious failure
		err := errors.New("Grease Grate clogged 💀")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		o11y.Logger.ErrorContext(childCtx, err.Error(), o11y.LoggerTraceAttr(ctx, span), o11y.LoggerSpanAttr(childCtx, span))

		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	} else {
		time.Sleep(3 * time.Millisecond) // artificial span increase
	}

	return nil
}

// InkWell checks whether we have depleted the ink
func InkWell(ctx context.Context, tracer trace.Tracer) error {
	childCtx, span := tracer.Start(ctx, "InkWell")
	defer span.End()

	// Whether to trip (between 0 and 1)
	tripValue := rand.Float64()
	// The threshold to trip the grease grate:
	// not below 0.9, increasing probablility from 0.9-1 and always above
	tripThreshold := float64(InkDepletion-90) / 10

	o11y.Logger.DebugContext(childCtx, fmt.Sprintf("inkDepletion %d - tripThreshold %f - tripValue %f", InkDepletion, tripThreshold, tripValue))

	if tripValue < tripThreshold {
		// this is a serious failure
		err := errors.New("Ink Well running dry 🐙")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		o11y.Logger.ErrorContext(childCtx, err.Error(), o11y.LoggerTraceAttr(ctx, span), o11y.LoggerSpanAttr(childCtx, span))

		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	} else {
		time.Sleep(3 * time.Millisecond) // artificial span increase
	}

	return nil
}
