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

package integration

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/FerretDB/FerretDB/integration/setup"
)

// TestUpdateProduceInfinity used to be TestDiffUpdateProduceInfinity: a $mul
// that overflows to +Inf was rejected here even though MongoDB itself allows
// it (setup.IsMongoDB(t) below always took the require.NoError(t, err)
// branch). That gap is closed - see internal/handler/common/update.go and
// TestInfinityDouble/Update/MulProducesInfinity in
// diff_05_document_validation_test.go for the equivalent insert/read
// round-trip - so this is a plain regression test now, not a documented
// difference from MongoDB.
func TestUpdateProduceInfinity(t *testing.T) {
	t.Parallel()

	ctx, collection := setup.Setup(t)
	_, err := collection.InsertOne(ctx, bson.D{{"_id", "number"}, {"v", int32(42)}})
	require.NoError(t, err)

	_, err = collection.UpdateOne(ctx, bson.D{{"_id", "number"}}, bson.D{{"$mul", bson.D{{"v", math.MaxFloat64}}}})
	require.NoError(t, err)

	var actual bson.M
	err = collection.FindOne(ctx, bson.D{{"_id", "number"}}).Decode(&actual)
	require.NoError(t, err)
	v, ok := actual["v"].(float64)
	require.True(t, ok)
	require.True(t, math.IsInf(v, +1))
}
