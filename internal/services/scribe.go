// Schildwächter's Genteel Beacon
// Copyright Carsten Thiel 2025-2026
//
// SPDX-Identifier: Apache-2.0

package services

import (
	"context"
	"math/rand/v2"
	"os"

	"github.com/schildwaechter/genteelbeacon/internal/config"
	"github.com/schildwaechter/genteelbeacon/internal/types"
	"go.opentelemetry.io/otel"
)

func FocusedScribe(ctx context.Context, requestId string) (types.CallingCard, error) {
	ctx, span := otel.Tracer(config.AppName).Start(ctx, "FocusedScribe")
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
	responseCallingCard.Attendant = config.AppName
	responseCallingCard.Salutation = randomNod
	responseCallingCard.CardVersion = config.BuildVersion
	responseCallingCard.Signature = nodeName
	responseCallingCard.Identifier = requestId

	return responseCallingCard, nil
}
