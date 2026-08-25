// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package stats

import (
	"context"
	"errors"
	"sync"
	"testing"

	"forgejo.org/models/db"
	"forgejo.org/modules/optional"
	"forgejo.org/modules/timeutil"

	"code.forgejo.org/xorm/xorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueueAndFlush(t *testing.T) {
	var mu sync.Mutex
	callValues := []int64{}
	RegisterRecalc(-99, func(ctx context.Context, i int64, _ optional.Option[timeutil.TimeStamp]) error {
		mu.Lock()
		defer mu.Unlock()
		callValues = append(callValues, i)
		return nil
	})

	safePush(t.Context(), recalcRequest{
		RecalcType: -99,
		ObjectID:   1,
	})

	require.NoError(t, Flush(t.Context()))
	func() {
		mu.Lock()
		defer mu.Unlock()
		assert.Len(t, callValues, 1)
		assert.EqualValues(t, 1, callValues[0])
	}()
}

func TestQueueUnique(t *testing.T) {
	var mu sync.Mutex
	callValues := []int64{}
	RegisterRecalc(-100, func(ctx context.Context, i int64, _ optional.Option[timeutil.TimeStamp]) error {
		mu.Lock()
		defer mu.Unlock()
		callValues = append(callValues, i)
		return nil
	})

	// Queue object with the same value multiple times... this test works OK with just 3 items, but with the queue
	// processing happening in the background it's possible that multiple invocations of the registered function can
	// happen.  So we'll test this by queuing a large number and ensuring that recalcs occurred less -- usually much
	// less, like once or twice.
	for range 300 {
		safePush(t.Context(), recalcRequest{
			RecalcType: -100,
			ObjectID:   1,
		})
	}

	require.NoError(t, Flush(t.Context()))
	func() {
		mu.Lock()
		defer mu.Unlock()
		assert.Less(t, len(callValues), 300)
		assert.EqualValues(t, 1, callValues[0])
	}()
}

func TestQueueAndError(t *testing.T) {
	var mu sync.Mutex
	callValues := []int64{}
	RegisterRecalc(-101, func(ctx context.Context, i int64, _ optional.Option[timeutil.TimeStamp]) error {
		mu.Lock()
		defer mu.Unlock()
		callValues = append(callValues, i)
		return errors.New("don't like that value")
	})

	safePush(t.Context(), recalcRequest{
		RecalcType: -101,
		ObjectID:   1,
	})

	for range 3 { // ensure object isn't requeued by flushing multiple times
		require.NoError(t, Flush(t.Context()))
	}
	func() {
		mu.Lock()
		defer mu.Unlock()
		assert.Len(t, callValues, 1)
		assert.EqualValues(t, 1, callValues[0])
	}()
}

func TestQueueAfterTx(t *testing.T) {
	// This is a really micro version of unittest.PrepareTestDatabase -- as the unittest package references the stats
	// package (for access to `Flush`), we can't use it without causing a circular dependency.  But we need a DB in
	// order to create a Tx.
	x, err := xorm.NewEngine("sqlite3", "file::memory:?cache=shared&_txlock=immediate")
	require.NoError(t, err)
	db.SetDefaultEngine(context.Background(), x)

	var mu sync.Mutex
	callValues := []int64{}
	RegisterRecalc(-102, func(ctx context.Context, i int64, _ optional.Option[timeutil.TimeStamp]) error {
		mu.Lock()
		defer mu.Unlock()
		callValues = append(callValues, i)
		return nil
	})

	err = db.WithTx(t.Context(), func(ctx context.Context) error {
		safePush(ctx, recalcRequest{
			RecalcType: -102,
			ObjectID:   1,
		})

		require.NoError(t, Flush(t.Context()))
		// Value from safePush() won't be sent yet because it was from within a DB transaction.
		assert.Empty(t, callValues)

		return nil
	})
	require.NoError(t, err)

	require.NoError(t, Flush(t.Context()))
	assert.Len(t, callValues, 1)
}
