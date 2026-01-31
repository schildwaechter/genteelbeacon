// Schildwächter's Genteel Beacon
// Copyright Carsten Thiel 2025-2026
//
// SPDX-Identifier: Apache-2.0

package services

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync/atomic"
	"time"

	"github.com/schildwaechter/genteelbeacon/internal/config"
	"github.com/schildwaechter/genteelbeacon/internal/o11y"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

var (
	// grease and ink tracking - using atomic for thread-safe access
	greaseBuildup atomic.Int64
	inkDepletion  atomic.Int64
	GreaseChan    chan int64
	InkChan       chan int64
)

// GetGreaseBuildup returns the current grease buildup value in a thread-safe manner
func GetGreaseBuildup() int64 {
	return greaseBuildup.Load()
}

// GetInkDepletion returns the current ink depletion value in a thread-safe manner
func GetInkDepletion() int64 {
	return inkDepletion.Load()
}

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
			if greaseChange == -1 && greaseBuildup.Load() > 0 {
				greaseBuildup.Add(-1)
				o11y.GreaseBuildupGaugeProm.Dec()
			} else if greaseChange == 1 && rand.IntN(100) < 40 {
				// we only increase grease buildup 40% to simulate different impact
				greaseBuildup.Add(1)
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
			if inkChange == -1 && inkDepletion.Load() > 0 {
				inkDepletion.Add(-1)
				o11y.InkDepletionGaugeProm.Dec()
			} else if inkChange == 1 {
				inkDepletion.Add(1)
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
func GreaseGrate(ctx context.Context) error {
	childCtx, span := otel.Tracer(config.AppName).Start(ctx, "GreaseGrate")
	defer span.End()

	// Whether to trip (between 0 and 1)
	tripValue := rand.Float64()
	// The threshold to trip the grease grate:
	// not below 0.9, increasing probability from 0.9-1 and always above
	currentGreaseBuildup := greaseBuildup.Load()
	tripThreshold := float64(currentGreaseBuildup-config.TripThreshold) / 10

	o11y.Logger.DebugContext(childCtx, fmt.Sprintf("greaseBuildup %d - tripThreshold %f - tripValue %f", currentGreaseBuildup, tripThreshold, tripValue))

	if tripValue < tripThreshold {
		// this is a serious failure
		err := errors.New("grease grate clogged 💀")
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
func InkWell(ctx context.Context) error {
	childCtx, span := otel.Tracer(config.AppName).Start(ctx, "InkWell")
	defer span.End()

	// Whether to trip (between 0 and 1)
	tripValue := rand.Float64()
	// The threshold to trip the grease grate:
	// not below 0.9, increasing probability from 0.9-1 and always above
	currentInkDepletion := inkDepletion.Load()
	tripThreshold := float64(currentInkDepletion-config.TripThreshold) / 10

	o11y.Logger.DebugContext(childCtx, fmt.Sprintf("inkDepletion %d - tripThreshold %f - tripValue %f", currentInkDepletion, tripThreshold, tripValue))

	if tripValue < tripThreshold {
		// this is a serious failure
		err := errors.New("ink well running dry 🐙")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		o11y.Logger.ErrorContext(childCtx, err.Error(), o11y.LoggerTraceAttr(ctx, span), o11y.LoggerSpanAttr(childCtx, span))

		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	} else {
		time.Sleep(3 * time.Millisecond) // artificial span increase
	}

	return nil
}
