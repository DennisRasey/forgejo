// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cache

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"forgejo.org/modules/log"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/sync"

	mc "code.forgejo.org/go-chi/cache"

	_ "code.forgejo.org/go-chi/cache/memcache" // memcache plugin for cache
)

var (
	conn             mc.Cache
	ErrInconvertible = errors.New("value from cache was not convertible to expected type")
	mutexMap         sync.MutexMap
)

func newCache(cacheConfig setting.Cache) (mc.Cache, error) {
	return mc.NewCacher(mc.Options{
		Adapter:       cacheConfig.Adapter,
		AdapterConfig: cacheConfig.Conn,
		Interval:      cacheConfig.Interval,
	})
}

// Init start cache service
func Init() error {
	var err error

	if conn == nil {
		if conn, err = newCache(setting.CacheService.Cache); err != nil {
			return err
		}
		if err = conn.Ping(); err != nil {
			return err
		}
	}

	return err
}

const (
	testCacheKey = "DefaultCache.TestKey"
)

func Test() (time.Duration, error) {
	if conn == nil {
		return 0, errors.New("default cache not initialized")
	}

	testData := fmt.Sprintf("%x", make([]byte, 500))

	start := time.Now()

	if err := conn.Delete(testCacheKey); err != nil {
		return 0, fmt.Errorf("expect cache to delete data based on key if exist but got: %w", err)
	}
	if err := conn.Put(testCacheKey, testData, 10); err != nil {
		return 0, fmt.Errorf("expect cache to store data but got: %w", err)
	}
	testVal := conn.Get(testCacheKey)
	if testVal == nil {
		return 0, errors.New("expect cache hit but got none")
	}
	if testVal != testData {
		return 0, errors.New("expect cache to return same value as stored but got other")
	}

	return time.Since(start), nil
}

// GetCache returns the currently configured cache
func GetCache() mc.Cache {
	return conn
}

// concurrencySafeGet is a single-process concurrency safe fetch from the cache, which provides the guarantee that after
// calling `cache.Remove(key)` and then `cache.Get*(key, ...)`, the value returned from cache will never have been
// computed **before** the `Remove` invocation.  It uses in-memory synchronization, so its guarantee does not extend to
// a clustered configuration.
//
// getFunc is the computation for the value if caching is not available.  convertFunc converts the cached value into the
// target type, and can return `ErrInconvertible` to indicate that the value couldn't be converts and should be
// recomputed instead; other errors are passed through.
func concurrencySafeGet[T any](key string, getFunc func() (T, error), convertFunc func(v any) (T, error)) (T, error) {
	if conn == nil || setting.CacheService.TTL <= 0 {
		return getFunc()
	}

	// Use a double-checking method -- once before acquiring the write lock on this key (this block), and then again
	// afterwards to avoid calling `getFunc` if it was computed while we were acquiring the lock. This causes two cache
	// hits as a trade-off to minimize the number of lock acquisitions. If this trade-off causes too much cache load,
	// this first `Get` could be removed -- the second one is performance-critical to ensure that after waiting a "long
	// time" to compute w/ `getFunc`, we don't immediately redo that work after acquiring the lock.
	cached := conn.Get(key)
	if cached != nil {
		retval, err := convertFunc(cached)
		if err == nil {
			return retval, nil
		} else if !errors.Is(err, ErrInconvertible) { // for ErrInconvertible we'll fall through to recalculating the value
			var zero T
			return zero, err
		}
	}

	defer mutexMap.Lock(key)()

	// The second, performance-critical, check if the cache contains the target value.
	cached = conn.Get(key)
	if cached != nil {
		retval, err := convertFunc(cached)
		if err == nil {
			return retval, nil
		} else if !errors.Is(err, ErrInconvertible) { // for ErrInconvertible we'll fall through to recalculating the value
			var zero T
			return zero, err
		}
	}

	value, err := getFunc()
	if err != nil {
		return value, err
	}
	return value, conn.Put(key, value, setting.CacheService.TTLSeconds())
}

// GetString returns the key value from cache with callback when no key exists in cache
func GetString(key string, getFunc func() (string, error)) (string, error) {
	v, err := concurrencySafeGet(key, getFunc, func(cached any) (string, error) {
		if value, ok := cached.(string); ok {
			return value, nil
		}
		if stringer, ok := cached.(fmt.Stringer); ok {
			return stringer.String(), nil
		}
		return fmt.Sprintf("%s", cached), nil
	})
	return v, err
}

// GetInt returns key value from cache with callback when no key exists in cache
func GetInt(key string, getFunc func() (int, error)) (int, error) {
	v, err := concurrencySafeGet(key, getFunc, func(cached any) (int, error) {
		switch v := cached.(type) {
		case int:
			return v, nil
		case string:
			value, err := strconv.Atoi(v)
			if err != nil {
				return 0, err
			}
			return value, nil
		}
		return 0, ErrInconvertible
	})
	return v, err
}

// GetInt64 returns key value from cache with callback when no key exists in cache
func GetInt64(key string, getFunc func() (int64, error)) (int64, error) {
	v, err := concurrencySafeGet(key, getFunc, func(cached any) (int64, error) {
		switch v := cached.(type) {
		case int64:
			return v, nil
		case string:
			value, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return 0, err
			}
			return value, nil
		}
		return 0, ErrInconvertible
	})
	return v, err
}

// Remove key from cache
func Remove(key string) {
	if conn == nil {
		return
	}

	// The goal of `Remove(key)` is to ensure that *after* it is completed, a new value is computed.  It's possible that
	// a value is being computed for the key *right now* -- `getFunc` is about to return, we're about to delete the key,
	// and then it will be Put into the cache with an out-of-date value computed before the `Remove(key)`. To prevent
	// this we need the `Remove(key)` to also lock on the key, just like `Get*(key, ...)` does when computing it.
	defer mutexMap.Lock(key)()

	err := conn.Delete(key)
	if err != nil {
		log.Error("unexpected error deleting key %s from cache: %v", err)
	}
}
