// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package callback

import (
	"context"
	"net/http"
	"time"

	"github.com/omec-project/openapi/v2/Nudr_DR"
	"github.com/omec-project/openapi/v2/models"
	udr_context "github.com/omec-project/udr/context"
	"github.com/omec-project/udr/logger"
)

const callbackRequestTimeout = 5 * time.Second

func closeCallbackResponseBody(httpResponse *http.Response) {
	if httpResponse != nil && httpResponse.Body != nil {
		if err := httpResponse.Body.Close(); err != nil {
			logger.HttpLog.Errorf("callback response body close failed: %v", err)
		}
	}
}

func SendOnDataChangeNotify(ueId string, notifyItems []models.NotifyItem) {
	udrSelf := udr_context.UDR_Self()

	for _, subscriptionDataSubscription := range udrSelf.SubscriptionDataSubscriptions {
		if ueId == subscriptionDataSubscription.GetUeId() {
			configuration := Nudr_DR.NewConfiguration()
			serverConfig := &configuration.Servers[0]
			if apiRootVar, exists := serverConfig.Variables["apiRoot"]; exists {
				apiRootVar.DefaultValue = subscriptionDataSubscription.GetCallbackReference()
				serverConfig.Variables["apiRoot"] = apiRootVar
			}
			client := Nudr_DR.NewAPIClient(configuration)

			dataChangeNotify := models.DataChangeNotify{
				UeId:                      &ueId,
				NotifyItems:               notifyItems,
				OriginalCallbackReference: []string{subscriptionDataSubscription.GetOriginalCallbackReference()},
			}
			ctx, cancel := context.WithTimeout(context.Background(), callbackRequestTimeout)
			apiOnDataChangeRequestBodyCallbackReferencePostRequest := client.SubsToNotifyCollectionCallbackDataChangeAPI.OnDataChangeRequestBodyCallbackReferencePost(ctx)
			apiOnDataChangeRequestBodyCallbackReferencePostRequest = apiOnDataChangeRequestBodyCallbackReferencePostRequest.Body(dataChangeNotify)
			httpResponse, err := client.SubsToNotifyCollectionCallbackDataChangeAPI.OnDataChangeRequestBodyCallbackReferencePostExecute(apiOnDataChangeRequestBodyCallbackReferencePostRequest)
			cancel()
			if err != nil {
				if httpResponse == nil {
					logger.HttpLog.Errorln(err.Error())
				} else if err.Error() != httpResponse.Status {
					logger.HttpLog.Errorln(err.Error())
				}
			}
			closeCallbackResponseBody(httpResponse)
		}
	}
}

func SendPolicyDataChangeNotification(policyDataChangeNotification []models.PolicyDataChangeNotification) {
	udrSelf := udr_context.UDR_Self()

	for _, policyDataSubscription := range udrSelf.PolicyDataSubscriptions {
		configuration := Nudr_DR.NewConfiguration()
		serverConfig := &configuration.Servers[0]
		if apiRootVar, exists := serverConfig.Variables["apiRoot"]; exists {
			apiRootVar.DefaultValue = policyDataSubscription.NotificationUri
			serverConfig.Variables["apiRoot"] = apiRootVar
		}
		client := Nudr_DR.NewAPIClient(configuration)

		ctx, cancel := context.WithTimeout(context.Background(), callbackRequestTimeout)
		apiPolicyDataChangeNotificationPostRequest := client.PolicyDataSubscriptionsCollectionCallbackpolicyDataChangeNotificationAPI.PolicyDataChangeNotificationPost(
			ctx)
		apiPolicyDataChangeNotificationPostRequest = apiPolicyDataChangeNotificationPostRequest.PolicyDataChangeNotification(policyDataChangeNotification)

		httpResponse, err := client.PolicyDataSubscriptionsCollectionCallbackpolicyDataChangeNotificationAPI.PolicyDataChangeNotificationPostExecute(apiPolicyDataChangeNotificationPostRequest)
		cancel()
		if err != nil {
			if httpResponse == nil {
				logger.HttpLog.Errorln(err.Error())
			} else if err.Error() != httpResponse.Status {
				logger.HttpLog.Errorln(err.Error())
			}
		}
		closeCallbackResponseBody(httpResponse)
	}
}
