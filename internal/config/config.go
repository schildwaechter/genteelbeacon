// Schildwächter's Genteel Beacon
// Copyright Carsten Thiel 2025-2026
//
// SPDX-Identifier: Apache-2.0

package config

import (
	"context"
	"log/slog"
	"os"
	"time"

	flagd "github.com/open-feature/go-sdk-contrib/providers/flagd/pkg"
	"github.com/open-feature/go-sdk/openfeature"
)

var (
	AppName      string
	GenteelRole  string
	NodeName     string
	BuildVersion string = "0.0.0" // should be overridden at compile time with -ldflags
	chaosMode    bool   = false
	chaosGates          = map[string]float64{
		"penDropChance":    0.01,
		"breakChance":      0.02,
		"indisposedChance": 0.04,
	}
)

// Service behavior constants
const (
	// Clerk error probabilities
	PenDropChance    = 0.01
	BreakChance      = 0.02
	IndisposedChance = 0.04

	// Threshold to start tripping ink/grease errors, integer percentage
	TripThreshold = 90
)

// GetEnv gets an environment variable with a default value
func GetEnv(name string, defaultValue string) string {
	value, exists := os.LookupEnv(name)
	if exists {
		return value
	}
	return defaultValue
}

func init() {
	AppName = GetEnv("GENTEEL_NAME", "Genteel Beacon")
	GenteelRole = GetEnv("GENTEEL_ROLE", "Default")
	// our name and role
	var err error
	NodeName, err = os.Hostname()
	if err != nil {
		NodeName = "unknown_host"
	}

	// Create a flagd provider pointing to your remote flagd server
	flagdHost := GetEnv("FLAGD_HOST", "")
	provider, err := flagd.NewProvider(
		flagd.WithHost(flagdHost), // flagd server address
		flagd.WithPort(8013),      // flagd port (default 8013)
	)
	if err != nil {
		slog.Error("Error creating flagd provider", "err", err)
	}

	// Set the global provider
	openfeature.SetProvider(provider)
	if flagdHost != "" {
		startsettingChaosMode()
	}

}

func startsettingChaosMode() {
	go func() {
		for {
			time.Sleep(5 * time.Second)
			client := openfeature.NewClient(AppName)
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)

			chaosModeVal, err := client.BooleanValue(ctx, "chaosMode", false, openfeature.EvaluationContext{})
			if err != nil {
				slog.Error("Error getting chaos mode value", "err", err)
				chaosMode = false
			}
			chaosMode = chaosModeVal
			cancel()
		}
	}()
}

func GetChaosChance(gate string) float64 {
	if chaosMode {
		client := openfeature.NewClient(AppName)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		penDropChanceVal, err := client.FloatValue(ctx, gate, chaosGates[gate], openfeature.EvaluationContext{})
		if err != nil {
			slog.Error("Error getting pen drop chance value", "err", err)
			return chaosGates[gate] // return default
		}
		return penDropChanceVal
	} else {
		return 0.0
	}
}
