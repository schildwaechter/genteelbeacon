// Schildwächter's Genteel Beacon
// Copyright Carsten Thiel 2025-2026
//
// SPDX-Identifier: Apache-2.0

// Package types defines the data structures used.
package types

type Telegram struct {
	Message        string
	Emoji          string
	FormVersion    string
	Service        string
	Telegraphist   string
	Identifier     string
	ClockReference string
}

type ClockReading struct {
	TimeReading string
	ClockName   string
}

type CallingCard struct {
	Attendant   string
	Salutation  string
	CardVersion string
	Signature   string
	Identifier  string
}
