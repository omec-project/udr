// Copyright (c) 2026 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package callback

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/omec-project/openapi/v2/models"
	udr_context "github.com/omec-project/udr/context"
)

func TestSendOnDataChangeNotifyUsesCallbackReferenceDirectly(t *testing.T) {
	udrSelf := udr_context.UDR_Self()
	originalSubscriptions := udrSelf.SubscriptionDataSubscriptions
	t.Cleanup(func() {
		udrSelf.SubscriptionDataSubscriptions = originalSubscriptions
	})

	requestPath := make(chan string, 1)
	requestBody := make(chan models.DataChangeNotify, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath <- r.URL.Path
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		var dataChangeNotify models.DataChangeNotify
		if err := json.Unmarshal(body, &dataChangeNotify); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}
		requestBody <- dataChangeNotify
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	callbackPath := "/notify/data-change"
	subscription := models.NewSubscriptionDataSubscriptions(server.URL+callbackPath, []string{"/nudr-dr/v2/subscription-data"})
	subscription.SetUeId("imsi-001010000000001")
	subscription.SetOriginalCallbackReference("original-callback")
	udrSelf.SubscriptionDataSubscriptions = map[string]*models.SubscriptionDataSubscriptions{
		"1": subscription,
	}

	SendOnDataChangeNotify("imsi-001010000000001", []models.NotifyItem{{ResourceId: "/subscription-data/imsi-001010000000001"}})

	if got := <-requestPath; got != callbackPath {
		t.Fatalf("expected request path %q, got %q", callbackPath, got)
	}
	body := <-requestBody
	if got := body.GetOriginalCallbackReference(); len(got) != 1 || got[0] != "original-callback" {
		t.Fatalf("expected original callback reference to be preserved, got %#v", got)
	}
}

func TestSendPolicyDataChangeNotificationUsesNotificationUriDirectly(t *testing.T) {
	udrSelf := udr_context.UDR_Self()
	originalSubscriptions := udrSelf.PolicyDataSubscriptions
	t.Cleanup(func() {
		udrSelf.PolicyDataSubscriptions = originalSubscriptions
	})

	requestPath := make(chan string, 1)
	requestBody := make(chan []models.PolicyDataChangeNotification, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath <- r.URL.Path
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		var notifications []models.PolicyDataChangeNotification
		if err := json.Unmarshal(body, &notifications); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}
		requestBody <- notifications
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	callbackPath := "/notify/policy-data"
	udrSelf.PolicyDataSubscriptions = map[string]*models.PolicyDataSubscription{
		"1": models.NewPolicyDataSubscription(server.URL+callbackPath, []string{"/nudr-dr/v2/policy-data"}),
	}

	notification := models.NewPolicyDataChangeNotification()
	notification.SetUeId("imsi-001010000000001")
	notifications := []models.PolicyDataChangeNotification{*notification}
	SendPolicyDataChangeNotification(notifications)

	if got := <-requestPath; got != callbackPath {
		t.Fatalf("expected request path %q, got %q", callbackPath, got)
	}
	if got := <-requestBody; len(got) != 1 || got[0].GetUeId() != notifications[0].GetUeId() {
		t.Fatalf("expected policy data change notification payload %#v, got %#v", notifications, got)
	}
}
