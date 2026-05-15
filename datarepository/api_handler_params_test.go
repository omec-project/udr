// Copyright (c) 2026 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package datarepository

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/omec-project/openapi/v2/models"
	udrContext "github.com/omec-project/udr/context"
)

func resetUDRContextForHandlerTests() {
	udrSelf := udrContext.UDR_Self()
	udrSelf.SubscriptionDataSubscriptions = make(map[string]*models.SubscriptionDataSubscriptions)
	udrSelf.EeSubscriptionIDGenerator = 1
	udrSelf.SubscriptionDataSubscriptionIDGenerator = 1
	udrSelf.UESubsCollection = sync.Map{}
	udrSelf.UriScheme = models.URISCHEME_HTTP
	udrSelf.RegisterIPv4 = "127.0.0.1"
	udrSelf.SBIPort = 8000
}

func TestHTTPCreateEeSubscriptions_UsesUeIdPathParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetUDRContextForHandlerTests()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost,
		"/subscription-data/imsi-001010000000001/context-data/ee-subscriptions",
		bytes.NewBufferString(`{}`),
	)
	request.Header.Set("Content-Type", "application/json")
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	context.Params = gin.Params{{Key: "ueId", Value: "imsi-001010000000001"}}

	HTTPCreateEeSubscriptions(context)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d with body %s", http.StatusCreated, recorder.Code, recorder.Body.String())
	}
	if _, ok := udrContext.UDR_Self().UESubsCollection.Load("imsi-001010000000001"); !ok {
		t.Fatal("expected subscription to be stored under ueId path param")
	}
	if _, ok := udrContext.UDR_Self().UESubsCollection.Load(""); ok {
		t.Fatal("did not expect subscription to be stored under empty ueId")
	}
}

func TestHTTPQueryeesubscriptions_UsesUeIdPathParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetUDRContextForHandlerTests()

	ueId := "imsi-001010000000001"
	udrContext.UDR_Self().UESubsCollection.Store(ueId, &udrContext.UESubsData{
		EeSubscriptionCollection: map[string]*udrContext.EeSubscriptionCollection{
			"1": {EeSubscriptions: &models.EeSubscription{}},
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet,
		"/subscription-data/"+ueId+"/context-data/ee-subscriptions",
		nil,
	)
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	context.Params = gin.Params{{Key: "ueId", Value: ueId}}

	HTTPQueryeesubscriptions(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d with body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
}

func TestHTTPRemovesubscriptionDataSubscriptions_UsesSubsIdPathParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetUDRContextForHandlerTests()

	subsId := "sub-1"
	udrContext.UDR_Self().SubscriptionDataSubscriptions[subsId] = &models.SubscriptionDataSubscriptions{}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodDelete,
		"/subscription-data/subs-to-notify/"+subsId,
		nil,
	)
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	context.Params = gin.Params{{Key: "subsId", Value: subsId}}

	HTTPRemovesubscriptionDataSubscriptions(context)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected %d, got %d with body %s", http.StatusNoContent, recorder.Code, recorder.Body.String())
	}
	if _, ok := udrContext.UDR_Self().SubscriptionDataSubscriptions[subsId]; ok {
		t.Fatal("expected subscription to be deleted using subsId path param")
	}
}
