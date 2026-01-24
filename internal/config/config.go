// Schildwächter's Genteel Beacon
// Copyright Carsten Thiel 2025
//
// SPDX-Identifier: Apache-2.0

package config

import "os"

var (
	AppName     string
	GenteelRole string
	NodeName    string
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
