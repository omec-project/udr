// SPDX-FileCopyrightText: 2026 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"maps"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// cacheTTL is the maximum age of a cached subscriber-data entry.
// Provisioned data changes only when simapp pushes updates; 30 s is
// conservative enough to be safe while still eliminating repeated MongoDB
// round-trips within a single UE registration flow.
const cacheTTL = 30 * time.Second

// cacheableCollections is the set of MongoDB collections whose
// RestfulAPIGetOne results are safe to cache. These are provisioned /
// authentication subscription data that is written by simapp before UEs
// connect and then read many times per registration.
var cacheableCollections = map[string]bool{
	"subscriptionData.provisionedData.amData":                        true,
	"subscriptionData.provisionedData.smfSelectionSubscriptionData":  true,
	"subscriptionData.provisionedData.smsData":                       true,
	"subscriptionData.provisionedData.smsMngData":                    true,
	"subscriptionData.provisionedData.traceData":                     true,
	"subscriptionData.authenticationData.authenticationSubscription": true,
}

type cacheEntry struct {
	data   map[string]any
	expiry time.Time
}

// cachedDBClient wraps a DBInterface, caching RestfulAPIGetOne for the
// provisioned collections listed in cacheableCollections. Write operations
// invalidate the relevant cache entry so callers always see consistent data
// when they write-then-read within the same process.
type cachedDBClient struct {
	DBInterface
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

func newCachedDBClient(inner DBInterface) *cachedDBClient {
	return &cachedDBClient{
		DBInterface: inner,
		entries:     make(map[string]cacheEntry),
	}
}

// cacheKey builds a stable key from the filter's ueId and servingPlmnId fields,
// which are the only fields used by cacheable collections.
func cacheKey(collName string, filter bson.M) string {
	ueId, _ := filter["ueId"].(string)
	plmnId, _ := filter["servingPlmnId"].(string)
	return collName + "\x00" + ueId + "\x00" + plmnId
}

func (c *cachedDBClient) RestfulAPIGetOne(collName string, filter bson.M) (map[string]any, error) {
	if !cacheableCollections[collName] {
		return c.DBInterface.RestfulAPIGetOne(collName, filter)
	}

	key := cacheKey(collName, filter)
	now := time.Now()

	c.mu.RLock()
	entry, hit := c.entries[key]
	c.mu.RUnlock()

	if hit && now.Before(entry.expiry) {
		return maps.Clone(entry.data), nil
	}

	data, err := c.DBInterface.RestfulAPIGetOne(collName, filter)
	if err != nil || data == nil {
		return data, err
	}

	c.mu.Lock()
	c.entries[key] = cacheEntry{data: maps.Clone(data), expiry: now.Add(cacheTTL)}
	c.mu.Unlock()

	return data, nil
}

func (c *cachedDBClient) invalidate(collName string, filter bson.M) {
	if !cacheableCollections[collName] {
		return
	}
	key := cacheKey(collName, filter)
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}

func (c *cachedDBClient) RestfulAPIPutOne(collName string, filter bson.M, putData map[string]any) (bool, error) {
	c.invalidate(collName, filter)
	return c.DBInterface.RestfulAPIPutOne(collName, filter, putData)
}

func (c *cachedDBClient) RestfulAPIPutOneTimeout(collName string, filter bson.M, putData map[string]any, timeout int32, timeField string) bool {
	c.invalidate(collName, filter)
	return c.DBInterface.RestfulAPIPutOneTimeout(collName, filter, putData, timeout, timeField)
}

func (c *cachedDBClient) RestfulAPIPutOneNotUpdate(collName string, filter bson.M, putData map[string]any) (bool, error) {
	c.invalidate(collName, filter)
	return c.DBInterface.RestfulAPIPutOneNotUpdate(collName, filter, putData)
}

func (c *cachedDBClient) RestfulAPIDeleteOne(collName string, filter bson.M) error {
	c.invalidate(collName, filter)
	return c.DBInterface.RestfulAPIDeleteOne(collName, filter)
}

func (c *cachedDBClient) RestfulAPIJSONPatch(collName string, filter bson.M, patchJSON []byte) error {
	c.invalidate(collName, filter)
	return c.DBInterface.RestfulAPIJSONPatch(collName, filter, patchJSON)
}

func (c *cachedDBClient) RestfulAPIMergePatch(collName string, filter bson.M, patchData map[string]any) error {
	c.invalidate(collName, filter)
	return c.DBInterface.RestfulAPIMergePatch(collName, filter, patchData)
}

func (c *cachedDBClient) RestfulAPIPost(collName string, filter bson.M, postData map[string]any) (bool, error) {
	c.invalidate(collName, filter)
	return c.DBInterface.RestfulAPIPost(collName, filter, postData)
}
