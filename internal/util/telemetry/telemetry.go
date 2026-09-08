// Copyright 2021 FerretDB Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package telemetry provides basic telemetry facilities.
package telemetry

import (
	"encoding"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlekSi/pointer"
)

// parseValue parses a string value into true, false, or nil.
func parseValue(s string) (*bool, error) {
	switch strings.ToLower(s) {
	case "1", "t", "true", "y", "yes", "on", "enable", "enabled", "optin", "opt-in", "allow":
		return pointer.ToBool(true), nil
	case "0", "f", "false", "n", "no", "off", "disable", "disabled", "optout", "opt-out", "disallow", "forbid":
		return pointer.ToBool(false), nil
	case "", "undecided":
		return nil, nil
	default:
		return nil, fmt.Errorf("failed to parse %s", s)
	}
}

// Flag represents a Kong flag with three states: true, false, and undecided (nil).
type Flag struct {
	v *bool
}

// UnmarshalText is used by Kong to parse a flag value.
func (s *Flag) UnmarshalText(text []byte) error {
	v, err := parseValue(string(text))
	if err != nil {
		return err
	}

	*s = Flag{v: v}

	return nil
}

// ForkNotice is the message shown wherever upstream FerretDB would otherwise
// report a telemetry state or point at where it sends telemetry. It is
// exported so every other package that used to echo a telemetry message
// (getLog, serverStatus, freeMonitoring, ...) shows the same wording instead
// of inventing its own.
const ForkNotice = "This is the wekan/FerretDB fork. Telemetry is completely removed: " +
	"no state is tracked, no report is ever built, and nothing is ever sent elsewhere."

// initialState always returns disabled and locked: this fork removes
// telemetry outright rather than merely defaulting it off, so there is no
// flag, environment variable, executable name, or previously saved state
// that can turn it back on. `f`, `dnt`, `execName` and `prev` are accepted
// only to keep this function's callers (and the CLI flags they come from)
// unchanged; none of them is read.
func initialState(f *Flag, dnt string, execName string, prev *bool, l *slog.Logger) (state *bool, locked bool, err error) {
	// Validate DO_NOT_TRACK the same way UnmarshalText validates the flag, so a
	// typo in it still fails startup instead of being silently ignored - even
	// though the parsed value can no longer change the (always-disabled) result.
	if _, err = parseValue(dnt); err != nil {
		return nil, false, err
	}

	if f.v != nil && *f.v {
		l.Warn("Telemetry cannot be enabled: " + ForkNotice)
	} else {
		l.Info(ForkNotice)
	}

	return pointer.ToBool(false), true, nil
}

// check interfaces
var (
	_ encoding.TextUnmarshaler = (*Flag)(nil)
)
