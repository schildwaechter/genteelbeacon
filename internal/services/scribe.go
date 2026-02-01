// Schildwächter's Genteel Beacon
// Copyright Carsten Thiel 2025-2026
//
// SPDX-Identifier: Apache-2.0

package services

import (
	"context"
	"math/rand/v2"
	"os"
	"time"

	"github.com/schildwaechter/genteelbeacon/internal/config"
	"github.com/schildwaechter/genteelbeacon/internal/o11y"
	"github.com/schildwaechter/genteelbeacon/internal/types"
	"go.opentelemetry.io/otel"
)

func FocusedScribe(ctx context.Context, requestID string) (types.CallingCard, error) {
	ctx, span := otel.Tracer(config.AppName).Start(ctx, "FocusedScribe")
	defer span.End()

	var responseCallingCard types.CallingCard
	nodeName, err := os.Hostname()
	if err != nil {
		nodeName = "unknown host"
	}

	scribeRandErrChance := rand.Float64()
	if scribeRandErrChance < config.GetChaosChance("penDropChance") { // very rare super long delay
		span.AddEvent("Pen search")
		o11y.Logger.WarnContext(ctx, "Scribe dropped the pen 🔍!!", o11y.LoggerTraceAttr(ctx, span), o11y.LoggerSpanAttr(ctx, span))
		time.Sleep(3 * time.Second) // uppss...
	} else {
		time.Sleep(time.Duration(rand.IntN(80)+50) * time.Millisecond) // normal artificial span increase
		span.AddEvent("Message ready")
	}

	nodOptions := []string{
		"A pleasure!", "Charmed!", "Delighted!", "Charmed, I'm sure!",
		"Quite so!", "Splendid!", "How lovely!", "My compliments!",
		"Pray tell!", "Fancy that!", "Always a joy!", "Quel plaisir!",
		"Enchantée!", "Très honorée!", "Très ravie!",
	}
	randomIndex := rand.IntN(len(nodOptions))
	randomNod := nodOptions[randomIndex]
	responseCallingCard.Attendant = config.AppName
	responseCallingCard.Salutation = randomNod
	responseCallingCard.CardVersion = config.BuildVersion
	responseCallingCard.Signature = nodeName
	responseCallingCard.Identifier = requestID

	return responseCallingCard, nil
}
