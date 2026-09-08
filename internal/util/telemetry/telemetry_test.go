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
	"testing"

	"github.com/AlekSi/pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FerretDB/FerretDB/internal/util/testutil"
)

// TestStateAlwaysDisabledAndLocked pins this fork's guarantee: telemetry is
// completely removed, not merely defaulted off, so no flag, environment
// variable, executable name, or previously saved state can produce anything
// other than disabled+locked. This replaces upstream's TestState, which
// exercised the flag/DNT/prev-state matrix that used to be able to turn
// telemetry on.
func TestStateAlwaysDisabledAndLocked(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		flag     string
		dnt      string
		execName string
		prev     *bool
	}{
		"default":          {},
		"prevEnabled":      {prev: pointer.ToBool(true)},
		"prevDisabled":     {prev: pointer.ToBool(false)},
		"flagEnable":       {flag: "enable"},
		"flagDisable":      {flag: "disable"},
		"dntSet":           {dnt: "1"},
		"execNameMentions": {execName: "donottrack"},
	} {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var f Flag
			require.NoError(t, f.UnmarshalText([]byte(tc.flag)))

			state, locked, err := initialState(&f, tc.dnt, tc.execName, tc.prev, testutil.Logger(t))
			require.NoError(t, err)
			assert.Equal(t, pointer.ToBool(false), state, "telemetry must never come back enabled")
			assert.True(t, locked, "telemetry must always be locked so nothing can enable it later")
		})
	}
}

// TestInvalidDNTStillRejected keeps upstream's one genuine input-validation
// case: parseValue still rejects a DO_NOT_TRACK value it cannot parse, even
// though the parsed value itself no longer affects the (always-disabled)
// outcome.
func TestInvalidDNTStillRejected(t *testing.T) {
	t.Parallel()

	var f Flag
	_, _, err := initialState(&f, "not-a-valid-value", "", nil, testutil.Logger(t))
	assert.EqualError(t, err, "failed to parse not-a-valid-value")
}
