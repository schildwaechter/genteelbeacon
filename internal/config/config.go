// Schildwächter's Genteel Beacon
// Copyright Carsten Thiel 2025-2026
//
// SPDX-Identifier: Apache-2.0

package config

import "os"

var (
	AppName      string
	GenteelRole  string
	NodeName     string
	BuildVersion string = "0.0.0" // should be overridden at compile time with -ldflags
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
}
