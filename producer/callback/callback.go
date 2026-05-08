// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package callback

import (
	"context"

	"github.com/omec-project/openapi/v2/Nudr_DR"
	"github.com/omec-project/openapi/v2/models"
	udr_context "github.com/omec-project/udr/context"
	"github.com/omec-project/udr/logger"
)

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
			apiOnDataChangeRequestBodyCallbackReferencePostRequest := client.SubsToNotifyCollectionCallbackDataChangeAPI.OnDataChangeRequestBodyCallbackReferencePost(context.TODO())
			apiOnDataChangeRequestBodyCallbackReferencePostRequest = apiOnDataChangeRequestBodyCallbackReferencePostRequest.Body(dataChangeNotify)
			httpResponse, err := client.SubsToNotifyCollectionCallbackDataChangeAPI.OnDataChangeRequestBodyCallbackReferencePostExecute(apiOnDataChangeRequestBodyCallbackReferencePostRequest)
			if err != nil {
				if httpResponse == nil {
					logger.HttpLog.Errorln(err.Error())
				} else if err.Error() != httpResponse.Status {
					logger.HttpLog.Errorln(err.Error())
				}
			}
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

		apiPolicyDataChangeNotificationPostRequest := client.PolicyDataSubscriptionsCollectionCallbackpolicyDataChangeNotificationAPI.PolicyDataChangeNotificationPost(
			context.TODO())
		apiPolicyDataChangeNotificationPostRequest = apiPolicyDataChangeNotificationPostRequest.PolicyDataChangeNotification(policyDataChangeNotification)

		httpResponse, err := client.PolicyDataSubscriptionsCollectionCallbackpolicyDataChangeNotificationAPI.PolicyDataChangeNotificationPostExecute(apiPolicyDataChangeNotificationPostRequest)
		if err != nil {
			if httpResponse == nil {
				logger.HttpLog.Errorln(err.Error())
			} else if err.Error() != httpResponse.Status {
				logger.HttpLog.Errorln(err.Error())
			}
		}
	}
}
