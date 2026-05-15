// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

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

func postCallbackJSON(ctx context.Context, callbackURI string, body any) (*http.Response, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, callbackURI, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")

	return http.DefaultClient.Do(request)
}

func SendOnDataChangeNotify(ueId string, notifyItems []models.NotifyItem) {
	udrSelf := udr_context.UDR_Self()

	for _, subscriptionDataSubscription := range udrSelf.SubscriptionDataSubscriptions {
		if ueId == subscriptionDataSubscription.GetUeId() {
			dataChangeNotify := models.DataChangeNotify{
				UeId:                      &ueId,
				NotifyItems:               notifyItems,
				OriginalCallbackReference: []string{subscriptionDataSubscription.GetOriginalCallbackReference()},
			}
			ctx, cancel := context.WithTimeout(context.Background(), callbackRequestTimeout)
			httpResponse, err := postCallbackJSON(ctx, subscriptionDataSubscription.GetCallbackReference(), dataChangeNotify)
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
		ctx, cancel := context.WithTimeout(context.Background(), callbackRequestTimeout)
		httpResponse, err := postCallbackJSON(ctx, policyDataSubscription.GetNotificationUri(), policyDataChangeNotification)
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
