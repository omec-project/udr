// SPDX-FileCopyrightText: 2026 Forsway Scandinavia AB
// SPDX-License-Identifier: Apache-2.0

package datarepository

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	udr_context "github.com/omec-project/udr/context"
)

// The producer-level tests in producer/data_repository_test.go drive
// CreateEeSubscriptionsProcedure directly. These drive the gin handlers instead, because
// that is the shape the defect actually arrives in: one goroutine per in-flight request,
// dispatched by the router, with no coordination between them. A guard that holds when the
// procedure is called in a loop but not when the handler is called concurrently would pass
// there and fail here.
//
// Two assertions, and the second is the one that needs stating. "It did not crash" is
// necessary but weak: `concurrent map writes` is a runtime fatal, so a failing run aborts
// the test binary rather than reporting, and a *lost* update or a *duplicated id* leaves no
// trace at all. So these also count what survived and check every id is distinct.
//
// PATCH and GET on the individual subscription are not exercised, because
// HTTPModifyEesubscription and HTTPQueryeeSubscription are stubs that return a
// not-implemented problem without touching the guarded state. Driving them would assert
// nothing about this fix.

func newEeRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/subscription-data/:ueId/context-data/ee-subscriptions", HTTPCreateEeSubscriptions)
	r.PUT("/subscription-data/:ueId/context-data/ee-subscriptions/:subsId", HTTPUpdateEesubscriptions)
	r.DELETE("/subscription-data/:ueId/context-data/ee-subscriptions/:subsId", HTTPRemoveeeSubscriptions)
	return r
}

// resetUeSubsCollection clears the process-wide singleton. Left populated, a second test in
// the same binary would see the first one's subscriptions and its count assertion would be
// wrong for reasons that have nothing to do with the code under test.
func resetUeSubsCollection(t *testing.T) {
	t.Helper()
	udrSelf := udr_context.UDR_Self()
	udrSelf.UESubsCollection.Range(func(key, _ any) bool {
		udrSelf.UESubsCollection.Delete(key)
		return true
	})
	udrSelf.EeSubscriptionIDGenerator.Store(1)
}

const eeSubscriptionBody = `{"callbackReference":"http://nef.example/notify","monitoringConfigurations":{}}`

func postEeSubscription(r *gin.Engine, ueId string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost,
		"/subscription-data/"+ueId+"/context-data/ee-subscriptions",
		strings.NewReader(eeSubscriptionBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// TestConcurrentEeSubscriptionPostsOverOneUeIdKeepIdsDistinct is the acceptance evidence for
// the HTTP path: many simultaneous creates against a single ueId, which is the case that
// aborted the process before the guard, and the case a non-atomic id counter silently
// corrupts.
func TestConcurrentEeSubscriptionPostsOverOneUeIdKeepIdsDistinct(t *testing.T) {
	resetUeSubsCollection(t)
	r := newEeRouter()

	const requests = 128
	const ueId = "imsi-208930100000001"

	var wg sync.WaitGroup
	locations := make([]string, requests)
	codes := make([]int, requests)
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w := postEeSubscription(r, ueId)
			codes[i] = w.Code
			locations[i] = w.Header().Get("Location")
		}(i)
	}
	wg.Wait()

	seen := make(map[string]int, requests)
	for i, loc := range locations {
		if codes[i] != http.StatusCreated {
			t.Fatalf("request %d: got status %d, want %d", i, codes[i], http.StatusCreated)
		}
		if loc == "" {
			t.Fatalf("request %d: no Location header, so the subscription id is unobservable", i)
		}
		seen[loc]++
	}
	// The duplicate-id failure is silent: every request succeeds, every response looks
	// right, and two subscriptions share an identity. Only comparing them catches it.
	for loc, n := range seen {
		if n != 1 {
			t.Errorf("Location %q was issued %d times; ids must be unique per subscription", loc, n)
		}
	}
	if len(seen) != requests {
		t.Errorf("got %d distinct ids from %d creates, want %d", len(seen), requests, requests)
	}
}

// TestConcurrentEeSubscriptionPostsAcrossUeIdsDoNotLoseEntries covers the other half: the
// per-UE entry is installed with LoadOrStore, and a Load/Store pair would let two creators
// for the same ueId install competing values so that one's subscriptions vanish. Spreading
// the load over several ueIds exercises the installation race as well as the map write.
func TestConcurrentEeSubscriptionPostsAcrossUeIdsDoNotLoseEntries(t *testing.T) {
	resetUeSubsCollection(t)
	r := newEeRouter()

	const ueCount = 8
	const perUe = 16

	var wg sync.WaitGroup
	for u := 0; u < ueCount; u++ {
		ueId := fmt.Sprintf("imsi-20893010002%04d", u)
		for i := 0; i < perUe; i++ {
			wg.Add(1)
			go func(ueId string) {
				defer wg.Done()
				if w := postEeSubscription(r, ueId); w.Code != http.StatusCreated {
					t.Errorf("ueId %s: got status %d, want %d", ueId, w.Code, http.StatusCreated)
				}
			}(ueId)
		}
	}
	wg.Wait()

	udrSelf := udr_context.UDR_Self()
	for u := 0; u < ueCount; u++ {
		ueId := fmt.Sprintf("imsi-20893010002%04d", u)
		value, ok := udrSelf.UESubsCollection.Load(ueId)
		if !ok {
			t.Fatalf("ueId %s: no entry at all, so every one of its creates was lost", ueId)
		}
		subs := value.(*udr_context.UESubsData)
		subs.Mtx.RLock()
		got := len(subs.EeSubscriptionCollection)
		subs.Mtx.RUnlock()
		if got != perUe {
			t.Errorf("ueId %s: kept %d subscriptions, want %d — the difference was lost silently",
				ueId, got, perUe)
		}
	}
}
