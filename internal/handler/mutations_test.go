// Copyright 2026 FerretDB contributors.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FerretDB/wire"
	"github.com/stretchr/testify/require"

	"github.com/FerretDB/FerretDB/internal/backends"
	"github.com/FerretDB/FerretDB/internal/backends/sqlite"
	"github.com/FerretDB/FerretDB/internal/clientconn/conninfo"
	"github.com/FerretDB/FerretDB/internal/clientconn/cursor"
	"github.com/FerretDB/FerretDB/internal/types"
	"github.com/FerretDB/FerretDB/internal/util/must"
	"github.com/FerretDB/FerretDB/internal/util/state"
	"github.com/FerretDB/FerretDB/internal/util/testutil"
)

// snapshotBackend pauses the first successful document read after the SQLite
// snapshot is materialized. A racing mutation would deterministically read the
// old document before the paused replacement is written.
type snapshotBackend struct {
	backends.Backend
	armed   atomic.Bool
	paused  chan struct{}
	release chan struct{}
}

func (b *snapshotBackend) Database(name string) (backends.Database, error) {
	db, err := b.Backend.Database(name)
	if err != nil {
		return nil, err
	}
	return &snapshotDatabase{Database: db, hook: b}, nil
}

type snapshotDatabase struct {
	backends.Database
	hook *snapshotBackend
}

func (d *snapshotDatabase) Collection(name string) (backends.Collection, error) {
	c, err := d.Database.Collection(name)
	if err != nil {
		return nil, err
	}
	return &snapshotCollection{Collection: c, hook: d.hook}, nil
}

type snapshotCollection struct {
	backends.Collection
	hook *snapshotBackend
}

func (c *snapshotCollection) Query(ctx context.Context, p *backends.QueryParams) (*backends.QueryResult, error) {
	result, err := c.Collection.Query(ctx, p)
	if err != nil {
		return nil, err
	}
	result.Iter = &snapshotIterator{DocumentsIterator: result.Iter, hook: c.hook}
	return result, nil
}

type snapshotIterator struct {
	types.DocumentsIterator
	hook *snapshotBackend
}

func (i *snapshotIterator) Next() (struct{}, *types.Document, error) {
	key, doc, err := i.DocumentsIterator.Next()
	if err == nil && doc != nil && i.hook.armed.CompareAndSwap(true, false) {
		close(i.hook.paused)
		<-i.hook.release
	}
	return key, doc, err
}

func TestMutationCommandsDoNotLoseConcurrentUpdates(t *testing.T) {
	// initCommands includes process-wide command metrics; restore their values
	// so this integration test does not change unrelated throttle expectations.
	beforeTotal := commandCounter.Load()
	beforeCounts := map[string]int64{}
	commandCounts.Range(func(k, v any) bool { beforeCounts[k.(string)] = v.(*atomic.Int64).Load(); return true })
	t.Cleanup(func() {
		commandCounter.Store(beforeTotal)
		commandCounts.Clear()
		for key, value := range beforeCounts {
			counter := new(atomic.Int64)
			counter.Store(value)
			commandCounts.Store(key, counter)
		}
	})

	for _, scenario := range []string{"AddToSet", "FilteredDisable", "FindAndModify"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := conninfo.Ctx(testutil.Ctx(t), conninfo.New())
			provider, err := state.NewProvider("")
			require.NoError(t, err)
			backend, err := sqlite.NewBackend(&sqlite.NewBackendParams{URI: testutil.TestSQLiteURI(t, ""), L: testutil.Logger(t), P: provider, BatchSize: 100})
			require.NoError(t, err)
			t.Cleanup(backend.Close)
			hook := &snapshotBackend{Backend: backend, paused: make(chan struct{}), release: make(chan struct{})}
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(hook.release) }) }
			t.Cleanup(unblock)
			h := &Handler{NewOpts: &NewOpts{L: testutil.Logger(t), BatchSize: 100}, b: hook, cursors: cursor.NewRegistry(testutil.Logger(t))}
			t.Cleanup(h.cursors.Close)
			h.initCommands()
			db, err := backend.Database("test")
			require.NoError(t, err)
			require.NoError(t, db.CreateCollection(ctx, &backends.CreateCollectionParams{Name: "accounts"}))
			coll, err := db.Collection("accounts")
			require.NoError(t, err)
			_, err = coll.InsertAll(ctx, &backends.InsertAllParams{Docs: []*types.Document{must.NotFail(types.NewDocument("_id", "account", "disabled", false, "tokens", must.NotFail(types.NewArray())))}})
			require.NoError(t, err)
			update := func(filter, change *types.Document) *wire.OpMsg {
				return must.NotFail(documentOpMsg(must.NotFail(types.NewDocument("update", "accounts", "updates", must.NotFail(types.NewArray(must.NotFail(types.NewDocument("q", filter, "u", change)))), "$db", "test"))))
			}
			filter := must.NotFail(types.NewDocument("_id", "account", "disabled", false))
			first := update(filter, must.NotFail(types.NewDocument("$addToSet", must.NotFail(types.NewDocument("tokens", "first")))))
			second := update(filter, must.NotFail(types.NewDocument("$addToSet", must.NotFail(types.NewDocument("tokens", "second")))))
			secondCommand := "update"
			if scenario == "FilteredDisable" {
				second = update(must.NotFail(types.NewDocument("_id", "account")), must.NotFail(types.NewDocument("$set", must.NotFail(types.NewDocument("disabled", true)))))
			}
			if scenario == "FindAndModify" {
				secondCommand = "findAndModify"
				second = must.NotFail(documentOpMsg(must.NotFail(types.NewDocument("findAndModify", "accounts", "query", filter, "update", must.NotFail(types.NewDocument("$addToSet", must.NotFail(types.NewDocument("tokens", "second")))), "$db", "test"))))
			}
			hook.armed.Store(true)
			firstResult := make(chan error, 1)
			secondResult := make(chan error, 1)
			go func() { _, err := h.commands["update"].Handler(ctx, first); firstResult <- err }()
			select {
			case <-hook.paused:
			case <-ctx.Done():
				t.Fatal("first update did not read")
			}
			go func() { _, err := h.commands[secondCommand].Handler(ctx, second); secondResult <- err }()
			// Without serialization the second operation completes while the first
			// still owns a stale snapshot. With the fix it waits until release.
			secondFinished := false
			select {
			case err := <-secondResult:
				require.NoError(t, err)
				secondFinished = true
			case <-time.After(100 * time.Millisecond):
			}
			unblock()
			require.NoError(t, <-firstResult)
			if !secondFinished {
				require.NoError(t, <-secondResult)
			}
			result, err := coll.Query(ctx, &backends.QueryParams{})
			require.NoError(t, err)
			defer result.Iter.Close()
			_, doc, err := result.Iter.Next()
			require.NoError(t, err)
			tokens := must.NotFail(doc.Get("tokens")).(*types.Array)
			if scenario == "FilteredDisable" {
				require.Equal(t, true, must.NotFail(doc.Get("disabled")), "stale token update re-enabled a disabled account")
				require.Equal(t, 1, tokens.Len())
				// After disable commits, the original conditional filter must match none.
				response, err := h.commands["update"].Handler(ctx, first)
				require.NoError(t, err)
				reply := must.NotFail(opMsgDocument(response))
				require.EqualValues(t, 0, must.NotFail(reply.Get("n")))
			} else {
				require.Equal(t, 2, tokens.Len(), "concurrent $addToSet discarded one acknowledged value")
			}
		})
	}
}

func TestMutationCommandGateCancellationAndReadBypass(t *testing.T) {
	h := &Handler{}
	called := atomic.Int32{}
	handler := func(context.Context, *wire.OpMsg) (*wire.OpMsg, error) { called.Add(1); return nil, nil }
	h.commands = map[string]*command{}
	mutations := []string{"update", "findAndModify", "findandmodify", "insert", "delete", "create", "drop", "dropDatabase", "renameCollection", "collMod", "createIndexes", "dropIndexes", "compact", "replSetInitiate", "createUser", "updateUser", "dropUser", "dropAllUsersFromDatabase"}
	reads := []string{"find", "getMore", "aggregate", "hello", "ping"}
	for _, name := range append(append([]string{}, mutations...), reads...) {
		h.commands[name] = &command{Handler: handler}
	}
	h.serializeMutationCommands()
	release, err := h.lockMutation(context.Background())
	require.NoError(t, err)
	defer release()
	for _, name := range mutations {
		canceled, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := h.commands[name].Handler(canceled, nil)
		require.ErrorIs(t, err, context.Canceled, name)
	}
	require.Zero(t, called.Load(), "canceled mutation reached its database handler")
	for _, name := range reads {
		finished := make(chan error, 1)
		go func() { _, err := h.commands[name].Handler(context.Background(), nil); finished <- err }()
		select {
		case err := <-finished:
			require.NoError(t, err)
		case <-time.After(time.Second):
			t.Fatalf("%s blocked behind mutation", name)
		}
	}
	require.EqualValues(t, len(reads), called.Load())
}

func TestMutationGateReleasedAfterFailure(t *testing.T) {
	h := &Handler{commands: map[string]*command{"update": {Handler: func(context.Context, *wire.OpMsg) (*wire.OpMsg, error) { return nil, errors.New("expected failure") }}}}
	h.serializeMutationCommands()
	_, err := h.commands["update"].Handler(context.Background(), nil)
	require.Error(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	release, err := h.lockMutation(ctx)
	require.NoError(t, err)
	release()
}

func TestBackgroundCleanupHonorsMutationGateCancellation(t *testing.T) {
	h := &Handler{}
	release, err := h.lockMutation(context.Background())
	require.NoError(t, err)
	defer release()
	for _, cleanup := range []func(context.Context) error{h.cleanupAllTTLCollections, h.cleanupAllCappedCollections} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.ErrorIs(t, cleanup(ctx), context.Canceled)
	}
}
