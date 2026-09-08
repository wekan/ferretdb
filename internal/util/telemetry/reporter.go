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

package telemetry

import (
	"context"
	"log/slog"
	"time"

	"github.com/FerretDB/FerretDB/internal/clientconn/connmetrics"
	"github.com/FerretDB/FerretDB/internal/util/state"
)

// Reporter used to send telemetry reports.
//
// Fork change: this fork removes telemetry completely rather than merely
// defaulting it off. Reporter keeps its original public shape (NewReporter,
// Run) so main.go and anything else built against this package keep
// compiling unchanged, but it never builds a request, never opens an HTTP
// client, and never performs a network call — see ForkNotice in
// telemetry.go, which initialState logs once at startup regardless of any
// flag, environment variable, or previously saved state.
type Reporter struct {
	*NewReporterOpts
}

// NewReporterOpts represents reporter options.
//
// The telemetry-specific fields (URL, UndecidedDelay, ReportInterval,
// ReportTimeout) are kept only so the CLI flags in cmd/ferretdb/main.go that
// populate them keep compiling; Reporter never reads them.
type NewReporterOpts struct {
	URL            string
	F              *Flag
	DNT            string
	ExecName       string
	P              *state.Provider
	ConnMetrics    *connmetrics.ConnMetrics
	L              *slog.Logger
	UndecidedDelay time.Duration
	ReportInterval time.Duration
	ReportTimeout  time.Duration
}

// NewReporter creates a new reporter. It always locks telemetry into the
// disabled state; see initialState in telemetry.go.
func NewReporter(opts *NewReporterOpts) (*Reporter, error) {
	t, locked, err := initialState(opts.F, opts.DNT, opts.ExecName, opts.P.Get().Telemetry, opts.L)
	if err != nil {
		return nil, err
	}

	err = opts.P.Update(func(s *state.State) {
		s.Telemetry = t
		s.TelemetryLocked = locked
	})
	if err != nil {
		return nil, err
	}

	return &Reporter{NewReporterOpts: opts}, nil
}

// Run does nothing and returns immediately: this fork never reports
// telemetry, so there is nothing to run periodically. It stays as a method
// (rather than being removed) so a caller that still invokes r.Run(ctx) -
// today none does, main.go leaves it uncalled on purpose - gets a no-op
// instead of a build failure or, worse, a reintroduced network call.
func (r *Reporter) Run(ctx context.Context) {
	r.L.DebugContext(ctx, ForkNotice)
}
