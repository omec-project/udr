// Copyright (c) 2026 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"maps"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// stubDB is a minimal DBInterface that counts RestfulAPIGetOne calls and
// returns a configurable result.
type stubDB struct {
	mu       sync.Mutex
	callsGet int
	result   map[string]any
	err      error
}

func (s *stubDB) getCallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.callsGet
}

func (s *stubDB) RestfulAPIGetOne(_ string, _ bson.M) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callsGet++
	// Return a fresh copy of the configured result each time.
	if s.result == nil {
		return nil, s.err
	}
	out := make(map[string]any, len(s.result))
	maps.Copy(out, s.result)
	return out, s.err
}

func (s *stubDB) RestfulAPIGetMany(_ string, _ bson.M) ([]map[string]any, error) {
	return nil, nil
}

func (s *stubDB) RestfulAPIPutOneTimeout(_ string, _ bson.M, _ map[string]any, _ int32, _ string) bool {
	return true
}

func (s *stubDB) RestfulAPIPutOne(_ string, _ bson.M, _ map[string]any) (bool, error) {
	return true, nil
}

func (s *stubDB) RestfulAPIPutOneNotUpdate(_ string, _ bson.M, _ map[string]any) (bool, error) {
	return true, nil
}
func (s *stubDB) RestfulAPIPutMany(_ string, _ []bson.M, _ []map[string]any) error { return nil }
func (s *stubDB) RestfulAPIDeleteOne(_ string, _ bson.M) error                     { return nil }
func (s *stubDB) RestfulAPIDeleteMany(_ string, _ bson.M) error                    { return nil }
func (s *stubDB) RestfulAPIMergePatch(_ string, _ bson.M, _ map[string]any) error  { return nil }
func (s *stubDB) RestfulAPIJSONPatch(_ string, _ bson.M, _ []byte) error           { return nil }
func (s *stubDB) RestfulAPIJSONPatchExtend(_ string, _ bson.M, _ []byte, _ string) error {
	return nil
}

func (s *stubDB) RestfulAPIPost(_ string, _ bson.M, _ map[string]any) (bool, error) {
	return true, nil
}
func (s *stubDB) RestfulAPIPostMany(_ string, _ bson.M, _ []interface{}) error { return nil }

const testColl = "subscriptionData.provisionedData.amData"

var testFilter = bson.M{"ueId": "imsi-001010000000001", "servingPlmnId": "00101"}

func TestCacheHitAvoidsDatabaseCall(t *testing.T) {
	db := &stubDB{result: map[string]any{"foo": "bar"}}
	c := newCachedDBClient(db)

	first, err := c.RestfulAPIGetOne(testColl, testFilter)
	if err != nil || first == nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}
	_, err = c.RestfulAPIGetOne(testColl, testFilter)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}

	if got := db.getCallCount(); got != 1 {
		t.Errorf("expected 1 DB call, got %d", got)
	}
}

func TestCacheExpiry(t *testing.T) {
	// Swap the TTL temporarily so the test does not wait 30 s.
	orig := cacheTTL
	// cacheTTL is a package-level const, so we work around it by directly
	// manipulating the entry's expiry via the internals after first load.
	_ = orig // const cannot be reassigned; we verify expiry by injecting a stale entry

	db := &stubDB{result: map[string]any{"foo": "bar"}}
	c := newCachedDBClient(db)

	// Populate the cache.
	if _, err := c.RestfulAPIGetOne(testColl, testFilter); err != nil {
		t.Fatal(err)
	}

	// Manually expire the entry.
	key := cacheKey(testColl, testFilter)
	c.mu.Lock()
	e := c.entries[key]
	e.expiry = time.Now().Add(-time.Second)
	c.entries[key] = e
	c.mu.Unlock()

	// Should hit DB again.
	if _, err := c.RestfulAPIGetOne(testColl, testFilter); err != nil {
		t.Fatal(err)
	}

	if got := db.getCallCount(); got != 2 {
		t.Errorf("expected 2 DB calls after expiry, got %d", got)
	}
}

func TestCacheInvalidatedOnPutOne(t *testing.T) {
	db := &stubDB{result: map[string]any{"foo": "bar"}}
	c := newCachedDBClient(db)

	if _, err := c.RestfulAPIGetOne(testColl, testFilter); err != nil {
		t.Fatal(err)
	}

	if _, err := c.RestfulAPIPutOne(testColl, testFilter, map[string]any{"foo": "new"}); err != nil {
		t.Fatal(err)
	}

	if _, err := c.RestfulAPIGetOne(testColl, testFilter); err != nil {
		t.Fatal(err)
	}

	if got := db.getCallCount(); got != 2 {
		t.Errorf("expected 2 DB calls (invalidation forces re-fetch), got %d", got)
	}
}

func TestCacheInvalidatedOnJSONPatch(t *testing.T) {
	db := &stubDB{result: map[string]any{"foo": "bar"}}
	c := newCachedDBClient(db)

	if _, err := c.RestfulAPIGetOne(testColl, testFilter); err != nil {
		t.Fatal(err)
	}

	if err := c.RestfulAPIJSONPatch(testColl, testFilter, []byte(`[]`)); err != nil {
		t.Fatal(err)
	}

	if _, err := c.RestfulAPIGetOne(testColl, testFilter); err != nil {
		t.Fatal(err)
	}

	if got := db.getCallCount(); got != 2 {
		t.Errorf("expected 2 DB calls after JSONPatch invalidation, got %d", got)
	}
}

func TestCacheReturnedMapIsolatedFromCachedCopy(t *testing.T) {
	db := &stubDB{result: map[string]any{"key": "original"}}
	c := newCachedDBClient(db)

	first, err := c.RestfulAPIGetOne(testColl, testFilter)
	if err != nil {
		t.Fatal(err)
	}
	// Mutate the returned map.
	first["key"] = "mutated"

	second, err := c.RestfulAPIGetOne(testColl, testFilter)
	if err != nil {
		t.Fatal(err)
	}
	if second["key"] != "original" {
		t.Errorf("cached entry was corrupted by caller mutation: got %q", second["key"])
	}
}

func TestNonCacheableCollectionBypassesCache(t *testing.T) {
	db := &stubDB{result: map[string]any{"foo": "bar"}}
	c := newCachedDBClient(db)

	for range 3 {
		if _, err := c.RestfulAPIGetOne("someOtherCollection", testFilter); err != nil {
			t.Fatal(err)
		}
	}

	if got := db.getCallCount(); got != 3 {
		t.Errorf("non-cacheable collection should always hit DB, got %d calls", got)
	}
}

func TestCacheConcurrentReads(t *testing.T) {
	db := &stubDB{result: map[string]any{"foo": "bar"}}
	c := newCachedDBClient(db)

	// Prime the cache.
	if _, err := c.RestfulAPIGetOne(testColl, testFilter); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			res, err := c.RestfulAPIGetOne(testColl, testFilter)
			if err != nil || res == nil {
				t.Errorf("concurrent read failed: err=%v res=%v", err, res)
			}
		})
	}
	wg.Wait()
}
