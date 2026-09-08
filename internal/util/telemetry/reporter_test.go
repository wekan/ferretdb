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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AlekSi/pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FerretDB/FerretDB/internal/clientconn/connmetrics"
	"github.com/FerretDB/FerretDB/internal/util/state"
	"github.com/FerretDB/FerretDB/internal/util/testutil"
)

// TestNewReporterAlwaysLocksDisabled replaces upstream's TestNewReporterLock:
// this fork's NewReporter locks telemetry into the disabled state no matter
// what the flag, DO_NOT_TRACK, or executable name say - see
// TestStateAlwaysDisabledAndLocked in telemetry_test.go for the same
// guarantee at the initialState level.
func TestNewReporterAlwaysLocksDisabled(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		f        *Flag
		dnt      string
		execName string
	}{
		"NoSet":      {f: new(Flag)},
		"FlagEnable": {f: &Flag{v: pointer.ToBool(true)}},
		"FlagDisable": {
			f: &Flag{v: pointer.ToBool(false)},
		},
		"DoNotTrack": {
			f:   new(Flag),
			dnt: "enable",
		},
		"ExecName": {
			f:        new(Flag),
			execName: "exec_donottrack",
		},
	} {
		name, tc := name, tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			sp, err := state.NewProvider("")
			require.NoError(t, err)

			opts := NewReporterOpts{
				F:           tc.f,
				DNT:         tc.dnt,
				ExecName:    tc.execName,
				ConnMetrics: connmetrics.NewListenerMetrics().ConnMetrics,
				P:           sp,
				L:           testutil.Logger(t),
			}

			_, err = NewReporter(&opts)
			assert.NoError(t, err)

			s := sp.Get()
			assert.Equal(t, pointer.ToBool(false), s.Telemetry)
			assert.True(t, s.TelemetryLocked)
		})
	}
}

// TestRunNeverCallsHome proves the actual removal: even pointing Reporter at
// a real, reachable HTTP server and running it to completion, no request
// ever reaches that server. This is the negative test for telemetry removal
// - it must hold regardless of where in the codebase Run/report is called
// from, not just at the one call site main.go happens to leave uncalled.
func TestRunNeverCallsHome(t *testing.T) {
	t.Parallel()

	var serverCalled int
	beacon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		serverCalled++
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(beacon.Close)

	sp, err := state.NewProvider("")
	require.NoError(t, err)

	opts := NewReporterOpts{
		URL:         beacon.URL,
		F:           &Flag{v: pointer.ToBool(true)}, // even an explicit (rejected) enable attempt
		ConnMetrics: connmetrics.NewListenerMetrics().ConnMetrics,
		P:           sp,
		L:           testutil.Logger(t),
	}

	r, err := NewReporter(&opts)
	require.NoError(t, err)

	r.Run(testutil.Ctx(t))

	assert.Equal(t, 0, serverCalled, "Run must never send a request to the telemetry endpoint")
}
