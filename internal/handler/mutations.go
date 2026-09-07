// Copyright 2026 FerretDB contributors.
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

package handler

import (
	"context"

	"github.com/FerretDB/wire"
)

// lockMutation protects the whole read/modify/write operation, not just its
// final SQL UPDATE. Backends receive replacement documents, so two operations
// that both read the same snapshot would otherwise discard one another's
// $inc/$addToSet changes even when the database serializes the SQL writes.
//
// The gate belongs to the shared server Handler, not an individual connection
// or transient Collection handle. Waiting honors client cancellation. This is
// single-server serialization, not a claim of cross-process transactions.
func (h *Handler) lockMutation(ctx context.Context) (func(), error) {
	h.mutationOnce.Do(func() { h.mutationGate = make(chan struct{}, 1) })
	select {
	case h.mutationGate <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-h.mutationGate
			return nil, err
		}
		return func() { <-h.mutationGate }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// serializeMutationCommands leaves reads (especially awaitable getMore,
// change streams and handshakes) outside the gate. $out/$merge aggregation
// stages are unsupported; if implemented they must join this mutation boundary.
// The wrapper runs before the existing command's first document query and is
// released only after its updates, deletes, or catalog changes complete.
func (h *Handler) serializeMutationCommands() {
	for _, name := range []string{
		"update", "findAndModify", "findandmodify", "insert", "delete",
		"create", "drop", "dropDatabase", "renameCollection", "collMod",
		"createIndexes", "dropIndexes", "compact", "replSetInitiate",
		"createUser", "updateUser", "dropUser", "dropAllUsersFromDatabase",
	} {
		cmd := h.commands[name]
		if cmd == nil {
			continue
		}
		inner := cmd.Handler
		cmd.Handler = func(ctx context.Context, msg *wire.OpMsg) (*wire.OpMsg, error) {
			release, err := h.lockMutation(ctx)
			if err != nil {
				return nil, err
			}
			defer release()
			return inner(ctx, msg)
		}
	}
}
