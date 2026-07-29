// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package cache

import (
	"sync"
)

// MutexMap is basically a map[string]sync.Mutex which allows you to have one mutex per string key being locked. Unlike
// a map[string]sync.Mutex, this map will automatically remove the Mutexes from itself when they are not being waited
// for, preventing resource waste. It does this by keeping a reference count of the current Lock calls for the given
// key.
type MutexMap struct {
	mu       sync.Mutex // mutex to be held when accessing mutexMap
	mutexMap map[string]*refcountMutex
}

type refcountMutex struct {
	refCount int // access to refCount is protected by the MutexMap's mu

	sync.Mutex
}

// Locks the given key, and returns a function that must be invoked to unlock the key and cleanup the mutex.
func (m *MutexMap) Lock(key string) func() {
	mutex := m.getOrCreateMutex(key)
	mutex.Lock()
	return m.makeUnlock(mutex, key)
}

// Attempts to lock the given key, and returns whether the key was locked, and a function that must be invoked to unlock
// the key and cleanup the mutex.  The returned function must be invoked even if the lock acqusition result was false.
func (m *MutexMap) TryLock(key string) (bool, func()) {
	mutex := m.getOrCreateMutex(key)
	lockAcquired := mutex.TryLock()
	if !lockAcquired {
		return false, func() {
			m.releaseMutex(mutex, key)
		}
	}
	return true, m.makeUnlock(mutex, key)
}

func (m *MutexMap) getOrCreateMutex(key string) *refcountMutex {
	m.mu.Lock()
	if m.mutexMap == nil {
		m.mutexMap = make(map[string]*refcountMutex)
	}
	mutex, ok := m.mutexMap[key]
	if !ok {
		mutex = &refcountMutex{}
		m.mutexMap[key] = mutex
	}
	mutex.refCount++
	m.mu.Unlock()

	return mutex
}

func (m *MutexMap) makeUnlock(mutex *refcountMutex, key string) func() {
	unlockPending := true

	return func() {
		if !unlockPending {
			// unlocking twice would cause incorrect reference counts and might release another goroutine's mutex -- try
			// to detect and panic so that this programming error can be found closest to the source.
			panic("MutexMap unlock invoked twice")
		}
		unlockPending = false
		mutex.Unlock()

		m.releaseMutex(mutex, key)
	}
}

func (m *MutexMap) releaseMutex(mutex *refcountMutex, key string) {
	m.mu.Lock()
	mutex.refCount--
	if mutex.refCount == 0 {
		delete(m.mutexMap, key)
	}
	m.mu.Unlock()
}
