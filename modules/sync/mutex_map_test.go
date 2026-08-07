// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package sync

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMutexMap_BasicLockUnlock(t *testing.T) {
	mm := &MutexMap{}

	unlock := mm.Lock("test-key")
	unlock()

	// Should be able to lock again
	unlock2 := mm.Lock("test-key")
	unlock2()
}

func TestMutexMap_BasicTryLockUnlock(t *testing.T) {
	mm := &MutexMap{}

	locked, unlock := mm.TryLock("test-key")
	assert.True(t, locked)
	unlock()

	// Should be able to lock again
	locked, unlock2 := mm.TryLock("test-key")
	assert.True(t, locked)
	unlock2()
}

func TestMutexMap_TryLock(t *testing.T) {
	mm := &MutexMap{}

	locked, unlock1 := mm.TryLock("test-key")
	defer unlock1()
	assert.True(t, locked)

	locked, unlock2 := mm.TryLock("test-key")
	defer unlock2()
	assert.False(t, locked)
}

func TestMutexMap_ConcurrentSameKey(t *testing.T) {
	mm := &MutexMap{}
	var anotherLockActive atomic.Bool
	var firstError atomic.Value
	var wg sync.WaitGroup

	for range 10 {
		wg.Go(func() {
			unlock := mm.Lock("shared-key")
			defer unlock()

			// should *not* find that another goroutine has put `true` into here.
			swapped := anotherLockActive.CompareAndSwap(false, true)
			if !swapped {
				firstError.CompareAndSwap(nil, "anotherLockActive was true!")
			}
			time.Sleep(time.Duration(rand.Intn(20)) * time.Millisecond) // jitter the goroutines to ensure no serial execution
			anotherLockActive.Store(false)
		})
	}

	wg.Wait()

	if err := firstError.Load(); err != nil {
		t.Fatal(err)
	}
}

func TestMutexMap_DifferentKeys(t *testing.T) {
	mm := &MutexMap{}
	done := make(chan bool, 1)

	go func() {
		// If these somehow referred to the same underlying `sync.Mutex`, because `sync.Mutex` is not re-entrant this would
		// never complete.
		unlock1 := mm.Lock("test-key-1")
		unlock2 := mm.Lock("test-key-2")
		unlock3 := mm.Lock("test-key-3")
		unlock1()
		unlock2()
		unlock3()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second): // early timeout so that we don't wait for t.Deadline()
		t.Fatal("test incomplete after timeout, indicating a locking bug")
	}
}

func TestMutexMap_SimpleCleanup(t *testing.T) {
	mm := &MutexMap{}
	unlock1 := mm.Lock("test-key-1")

	mm.mu.Lock()
	assert.Len(t, mm.mutexMap, 1)
	mm.mu.Unlock()

	unlock1()

	mm.mu.Lock()
	assert.Empty(t, mm.mutexMap)
	mm.mu.Unlock()
}

func TestMutexMap_TryLockCleanup(t *testing.T) {
	mm := &MutexMap{}
	_, unlock1 := mm.TryLock("test-key-1")
	_, unlock2 := mm.TryLock("test-key-1")

	mm.mu.Lock()
	assert.Len(t, mm.mutexMap, 1)
	mm.mu.Unlock()

	unlock1()
	unlock2()

	mm.mu.Lock()
	assert.Empty(t, mm.mutexMap)
	mm.mu.Unlock()
}

func TestMutexMap_ConcurrentCleanup(t *testing.T) {
	mm := &MutexMap{}
	var foundRefGreaterThanOne atomic.Bool
	var wg sync.WaitGroup

	for range 10 {
		wg.Go(func() {
			unlock := mm.Lock("shared-key")
			defer unlock()

			time.Sleep(time.Duration(rand.Intn(20)) * time.Millisecond) // jitter the goroutines to ensure no serial execution

			mm.mu.Lock()
			rcMutex := mm.mutexMap["shared-key"]
			if rcMutex.refCount > 1 {
				foundRefGreaterThanOne.Store(true)
			}
			mm.mu.Unlock()
		})
	}

	wg.Wait()

	assert.True(t, foundRefGreaterThanOne.Load(), "expected to find a refCount > 1")

	mm.mu.Lock()
	assert.Empty(t, mm.mutexMap)
	mm.mu.Unlock()
}

func TestMutexMap_UnlockTwice(t *testing.T) {
	mm := &MutexMap{}
	assert.Panics(t, func() {
		unlock := mm.Lock("test")
		unlock()
		unlock()
	})
}
